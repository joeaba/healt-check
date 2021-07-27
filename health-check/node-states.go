package solanahc

import (
	"errors"
	"log"
	"sync"
	"time"

	solanarpc "github.com/linuskendall/solana-rpc-health-check/rpc"
)

var (
	RpcTimeout = 10 * time.Second
)

type NodeStates struct {
	States         []NodeState
	nodes          []string
	LoadBlocks     bool
	LoadLedgerSize bool
}

// Run a health check on a list of nodes, returnign a set of nodestates
func (ns *NodeStates) LoadStates() (n_states int, err error) {
	if len(ns.nodes) > 0 {
		var waitgroup sync.WaitGroup

		// Channel to receive the nodestates when they're done
		st := make(chan *NodeState, len(ns.nodes))

		// Load the health data for each of the nodes
		for i := 0; i < len(ns.nodes); i++ {
			waitgroup.Add(1)
			go func(i int) {
				defer waitgroup.Done()
				state := NewNodeState(ns.nodes[i])

				state.LoadEpoch()
				if state.HasErrors {
					return
				}

				state.LoadSlots()

				if ns.LoadLedgerSize {
					state.LoadMinimumLedger()
				}
				if ns.LoadBlocks {
					state.LoadBlocks()
				}

				st <- state
			}(i)
		}

		waitgroup.Wait()
		close(st)

		// Recreate the states
		ns.States = make([]NodeState, 0)
		for s := range st {
			if s.HasErrors {
				log.Println("state has errors, ignoring=", s.RpcNode)
			} else {
				log.Println("loaded state=", s.RpcNode, s.CurrentSlot)
				ns.States = append(ns.States, *s)
			}
		}

		if len(ns.States) < 2 {
			err = errors.New("couldn't fetch more than one node")
		}
	} else {
		err = errors.New("need at least one node")
	}
	n_states = len(ns.States)
	return
}

// Get healthy state from a list of nodestates
func (ns *NodeStates) GetHealthyState() (currentSlot solanarpc.Slot, prevMaxBlocks int, curMaxBlocks int, err error) {
	if len(ns.States) < 1 {
		err = errors.New("need at least one node state to create the health states")
		return
	}

	// Iterate all other elements in loop
	for sx1 := 0; sx1 < len(ns.States); sx1++ {
		if ns.States[sx1].CurrentSlot > currentSlot {
			currentSlot = ns.States[sx1].CurrentSlot
		}

		if ns.LoadBlocks {
			prevEpochBlocks := len(ns.States[sx1].PrevEpochBlocks)
			curEpochBlocks := len(ns.States[sx1].CurEpochBlocks)
			if prevEpochBlocks > prevMaxBlocks {
				prevMaxBlocks = prevEpochBlocks
			}
			if curEpochBlocks > curMaxBlocks {
				curMaxBlocks = curEpochBlocks
			}
		}
		log.Println("healthy states=", sx1, ns.States[sx1].RpcNode, ns.States[sx1].CurrentSlot)
	}

	return
}

// Constructor
func NewNodeStates(nodes []string, loadBlocks bool, loadLedgerSize bool) *NodeStates {
	return &NodeStates{
		nodes:          nodes,
		LoadBlocks:     loadBlocks,
		LoadLedgerSize: loadLedgerSize,
	}
}
