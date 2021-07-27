package main

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	solanahc "github.com/linuskendall/solana-rpc-health-check/health-check"
)

type Status string

const (
	Up   = Status("up")
	Down = Status("down")
)

type HealthState struct {
	// These are never changed
	RpcUri            string
	Servers           []string
	BlockCheckEnabled bool
	MinimumLedgerSize uint64

	// Mu is the state mutex
	mu         sync.RWMutex
	nodeStates *solanahc.NodeStates

	// Ms is the status mutex
	ms            sync.RWMutex
	last_failure  string
	status        Status
	load_failures uint64
	fall          uint64
	rise          uint64
}

// This method continuously updates the node state
func (s *HealthState) UpdateState(schedule time.Duration) {
	ticker := time.NewTicker(schedule)

	for {
		log.Println("checking servers ", s.Servers)
		s.mu.Lock()
		n_states, err := s.nodeStates.LoadStates()
		s.mu.Unlock()

		if err != nil {
			log.Println("error loading states ", err)
			s.RegisterLoadFailure("loadinghc")
		} else {
			// Reset load failures counter
			atomic.StoreUint64(&s.load_failures, 0)

			log.Println("saved ", n_states, " states")

			// Check the state
			s.CheckHealth()
		}

		<-ticker.C
	}
}

func (s *HealthState) GetState() (rpc_state solanahc.NodeState, other_node_states []solanahc.NodeState) {
	s.mu.RLock()
	for _, state := range s.nodeStates.States {
		switch state.RpcNode {
		case s.RpcUri:
			if state.HasErrors {
				fmt.Println("warning! ", state.RpcNode, " has errors, including it")
			}
			rpc_state = state
		default:
			if state.HasErrors {
				fmt.Println("warning! ", state.RpcNode, " has errors, ignoring it")
			} else {
				other_node_states = append(other_node_states, state)
			}
			break
		}
	}
	s.mu.RUnlock()
	return
}

func (s *HealthState) CheckHealth() {
	// Get a relevant copy of the state to work on
	rpc_state, other_node_states := s.GetState()

	log.Println("number of states: ", len(other_node_states))

	// Check that we have at least one node to compare to
	// in case the user has provided reference servers
	if len(s.Servers) > 1 && len(other_node_states) < 1 {
		log.Println("insufficient comparison states loaded")
		s.RegisterLoadFailure("lacksstates")
		return
	}

	// If we can't actually load the node, lets register it down immediately
	if rpc_state.RpcNode != s.RpcUri {
		log.Println("error server not found")
		s.RegisterDownImmediate("notfound")
		return
	}

	// If it has an error, maybe register immediately? Not certain.
	if rpc_state.HasErrors {
		log.Println("error couldn't load the current rpc state")
		s.RegisterDown("checkerror")
		return
	}

	currentSlot, prevBlocks, curBlocks, err := s.nodeStates.GetHealthyState()
	if err != nil {
		log.Println("error couldn't load healthy state")
		s.RegisterLoadFailure("healthstate")
		return
	}

	log.Println("**", "checking the health status of: ", rpc_state.RpcNode)

	var failures []string

	// This check compares the slot of reference servers to itself
	if len(s.Servers) > 1 {
		compareCurrentSlot := int64(rpc_state.CurrentSlot - currentSlot)
		log.Println("***", "compareCurrentSlot: remote=", currentSlot, "local=", rpc_state.CurrentSlot, "diff=", compareCurrentSlot)

		if compareCurrentSlot < -int64(*MAX_SLOT_DIFF) {
			log.Println("node is unhealthy, it is more than ", *MAX_SLOT_DIFF, " slots behind")
			failures = append(failures, "behind")
		}
	}

	if *MAX_TRANSMIT_CHECK_ENABLED {
		compareMaxTransmit := int64(rpc_state.CurrentSlot - rpc_state.MaxRetransmitSlot)
		log.Println("***", "compareMaxTransmit: remote=", rpc_state.MaxRetransmitSlot, "local=", rpc_state.CurrentSlot, "diff=", compareMaxTransmit)

		if compareMaxTransmit < -int64(*MAX_SLOT_DIFF) {
			//log.Println("[currently disabled transmit check] node is unhealthy, it is more than ", *MAX_SLOT_DIFF, " slots behind")
			//failures = append(failures, "max-retransmit")
		}
	}

	if *MINIMUM_LEDGER_SIZE > 0 {
		slotsStored := uint64(rpc_state.CurrentSlot - rpc_state.MinimumSlot)
		log.Println("***", "checkSlotsStored: healthy=", *MINIMUM_LEDGER_SIZE, "local=", slotsStored)

		if slotsStored < uint64(*MINIMUM_LEDGER_SIZE) {
			log.Println("node is unhealthy, it does not have ", *MINIMUM_LEDGER_SIZE, " slots stored")
			failures = append(failures, "slotsstored")
		}
	}

	if *BLOCK_CHECK_ENABLED {
		currentEpochBlocks := len(rpc_state.CurEpochBlocks)
		prevEpochBlocks := len(rpc_state.PrevEpochBlocks)
		currentEpochBlockDiff := currentEpochBlocks - curBlocks
		prevEpochBlockDiff := prevEpochBlocks - prevBlocks
		log.Println("***", "blockCheck (current epoch): healthy=", curBlocks, " local=", currentEpochBlocks, " diff=", currentEpochBlockDiff)
		log.Println("***", "blockCheck (previous epoch): healthy=", prevBlocks, " local=", prevEpochBlocks, " diff=", prevEpochBlockDiff)

		if currentEpochBlocks <= 0 {
			log.Println("node is unhealthy, there are holes in the current epoch block records")
			failures = append(failures, "holes")
		} else if currentEpochBlockDiff < -int(*MAX_BLOCK_DIFF) || currentEpochBlockDiff > int(*MAX_BLOCK_DIFF) {
			log.Println("node is unhealthy, there is a difference of ", currentEpochBlockDiff, " which is more than ", int(*MAX_BLOCK_DIFF))
			failures = append(failures, "blockdiff")
		}
	}
	if len(failures) > 0 {
		log.Println("registering down")
		s.RegisterDown(strings.Join(failures, ","))
	} else {
		log.Println("registering up")
		s.RegisterUp()
	}
	return
}

func (s *HealthState) RegisterLoadFailure(failure string) {
	log.Println("load failure", failure)

	s.ms.Lock()
	s.last_failure = failure
	s.ms.Unlock()

	atomic.AddUint64(&s.load_failures, 1)
}

func (s *HealthState) RegisterDownImmediate(failure string) {
	s.ms.Lock()
	log.Println("registering immediately down")
	s.status = Down
	s.ms.Unlock()

	atomic.StoreUint64(&s.rise, 0)
	atomic.StoreUint64(&s.fall, 0)
}

func (s *HealthState) RegisterDown(failure string) {
	s.ms.Lock()
	s.last_failure = failure
	stat := s.status
	s.ms.Unlock()

	if stat == Up {
		fall := atomic.AddUint64(&s.fall, 1)
		log.Println("fall ", atomic.LoadUint64(&s.fall))
		// There has been more than DOWN_THRESHOLD consecutive invalid health checks
		// change state to down and reset counters
		if fall >= uint64(*DOWN_THRESHOLD) {
			s.ms.Lock()
			s.status = Down
			s.ms.Unlock()
			atomic.StoreUint64(&s.rise, 0)
			atomic.StoreUint64(&s.fall, 0)
		}
	} else {
		atomic.StoreUint64(&s.rise, 0)
		atomic.StoreUint64(&s.fall, 0)
	}
}

func (s *HealthState) RegisterUp() {
	s.ms.Lock()
	s.last_failure = ""
	stat := s.status
	s.ms.Unlock()

	if stat == Down {
		rise := atomic.AddUint64(&s.rise, 1)
		log.Println("rise ", atomic.LoadUint64(&s.rise))
		// There has been more than UP_THRESHOLD consecutive valid health checks
		// change state to up and reset counters
		if rise >= uint64(*UP_THRESHOLD) {
			s.ms.Lock()
			s.status = Up
			s.ms.Unlock()
			atomic.StoreUint64(&s.rise, 0)
			atomic.StoreUint64(&s.fall, 0)
		}
	} else {
		atomic.StoreUint64(&s.rise, 0)
		atomic.StoreUint64(&s.fall, 0)
	}
}

func (s *HealthState) IsStale() bool {
	if atomic.LoadUint64(&s.load_failures) > 3 {
		return true
	} else {
		return false
	}
}

func (s *HealthState) GetStatus() (status string) {
	if s.IsStale() {
		status = string(Down) + " #stale"
		return
	}

	s.ms.RLock()
	if s.status == "" {
		status = string(Down)
	} else if s.last_failure != "" {
		status = string(s.status) + " #" + s.last_failure
	} else {
		status = string(s.status)
	}
	s.ms.RUnlock()
	return
}

func NewHealthState(rpcUri string, reference_servers []string) *HealthState {
	serverList := append([]string{*rpcURI}, reference_servers...)
	ledgerCheck := (*MINIMUM_LEDGER_SIZE > 0)

	return &HealthState{
		RpcUri:     rpcUri,
		Servers:    serverList,
		nodeStates: solanahc.NewNodeStates(serverList, *BLOCK_CHECK_ENABLED, ledgerCheck),
		status:     Down,
	}
}
