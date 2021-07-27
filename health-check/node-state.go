package solanahc

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"

	"github.com/linuskendall/solana-rpc-health-check/rpc"
	solanarpc "github.com/linuskendall/solana-rpc-health-check/rpc"
)

type NodeState struct {
	client            *solanarpc.Client
	RpcNode           string
	HasErrors         bool
	Errors            []error
	MinimumSlot       rpc.Slot
	CurrentSlot       rpc.Slot
	ProcessedSlot     rpc.Slot
	MaxRetransmitSlot rpc.Slot
	Version           rpc.Version
	Identity          rpc.Identity
	PrevEpochBlocks   []uint64
	CurEpochBlocks    []uint64
	Epoch             solanarpc.EpochInfo
	EpochSchedule     solanarpc.EpochSchedule
	GenesisHash       string
	epochLoaded       bool
}

// @TODO put into goroutines
func (state *NodeState) LoadMeta() (err error) {
	var err2, err3 error

	ctx, cancel := context.WithTimeout(context.Background(), RpcTimeout)
	state.Version, err = state.client.GetVersion(ctx)
	cancel()

	if err != nil {
		state.HasErrors = true
		state.Errors = append(state.Errors, err)
	}

	ctx, cancel = context.WithTimeout(context.Background(), RpcTimeout)
	state.Identity, err2 = state.client.GetIdentity(ctx)
	cancel()

	if err2 != nil {
		state.HasErrors = true
		state.Errors = append(state.Errors, err)
		err = err2
	}

	ctx, cancel = context.WithTimeout(context.Background(), RpcTimeout)
	state.GenesisHash, err3 = state.client.GetGenesisHash(ctx)
	cancel()

	if err3 != nil {
		state.HasErrors = true
		state.Errors = append(state.Errors, err)
		err = err3
	}

	return
}

// Loads Epoch details
func (state *NodeState) LoadEpoch() (err error) {
	ctx, cancel := context.WithTimeout(context.Background(), RpcTimeout)
	state.Epoch, err = state.client.GetEpochInfo(ctx, "")
	cancel()

	if err != nil {
		state.HasErrors = true
		state.Errors = append(state.Errors, err)
		return
	}

	ctx, cancel = context.WithTimeout(context.Background(), RpcTimeout)
	state.EpochSchedule, err = state.client.GetEpochSchedule(ctx)
	cancel()

	if err != nil {
		state.HasErrors = true
		state.Errors = append(state.Errors, err)
		return
	}

	state.epochLoaded = true
	return
}

// Runs the RPC calls for a single node
func (state *NodeState) LoadMinimumLedger() (err error) {
	ctx, cancel := context.WithTimeout(context.Background(), RpcTimeout)
	defer cancel()

	state.MinimumSlot, err = state.client.MinimumLedgerSlot(ctx)

	// we log the errors here but we don't cause any fuirther error hanlding
	if err != nil {
		log.Println(err)
		state.HasErrors = true
		state.Errors = append(state.Errors, err)
	}

	return
}

func (state *NodeState) LoadBlocks() (err error) {
	rpc_errors := make(chan error, 4)

	if !state.epochLoaded {
		rpc_errors <- NewError(state.RpcNode, errors.New("Epoch not loaded, can't request blocks"))
		return
	}

	currentEpoch := state.Epoch.Epoch
	previousEpoch := state.Epoch.Epoch - solanarpc.Epoch(1)

	var waitgroup sync.WaitGroup
	waitgroup.Add(2)

	go func() {
		defer waitgroup.Done()
		ctx, cancel := context.WithTimeout(context.Background(), RpcTimeout)
		defer cancel()

		// Check if we have slots from this epoch
		first_slot := state.EpochSchedule.GetFirstSlotInEpoch(previousEpoch)
		last_slot := state.EpochSchedule.GetLastSlotInEpoch(previousEpoch)
		if state.MinimumSlot != 0 {
			if last_slot < state.MinimumSlot {
				//rpc_errors <- NewError(state.RpcNode, errors.New("no slots stored from previous epoch"))
				log.Println("no slots stored from previous epoch, ignoring")
				return
			} else if first_slot < state.MinimumSlot {
				first_slot = state.MinimumSlot
				log.Println(fmt.Sprintf("[%s] minimum slot below previous epoch first slot available, changing first slot to %d", state.RpcNode, state.MinimumSlot))
			}
		}

		// Load blocks
		var err error
		state.PrevEpochBlocks, err = state.client.GetConfirmedBlocks(ctx, first_slot, last_slot)
		if err != nil {
			rpc_errors <- err
		}
	}()

	go func() {
		defer waitgroup.Done()
		ctx, cancel := context.WithTimeout(context.Background(), RpcTimeout)
		defer cancel()

		// Check if we have slots from this epoch
		first_slot := state.EpochSchedule.GetFirstSlotInEpoch(currentEpoch)
		last_slot := state.EpochSchedule.GetLastSlotInEpoch(currentEpoch)

		if state.MinimumSlot != 0 {
			if last_slot < state.MinimumSlot {
				//rpc_errors <- NewError(state.RpcNode, errors.New("no slots stored from current epoch"))
				log.Println("no slots stored from current epoch, ignoring")
				return
			} else if first_slot < state.MinimumSlot {
				first_slot = state.MinimumSlot
				log.Println(fmt.Sprintf("[%s] minimum slot below current epoch first slot available, changing first slot to %d", state.RpcNode, state.MinimumSlot))
			}
		}

		// Load blocks
		var err error
		state.CurEpochBlocks, err = state.client.GetConfirmedBlocks(ctx, first_slot, last_slot)
		if err != nil {
			rpc_errors <- err
		}
	}()

	waitgroup.Wait()
	close(rpc_errors)

	// we log the errors here but we don't cause any fuirther error hanlding
	if len(rpc_errors) > 0 {
		state.HasErrors = true
		for err = range rpc_errors {
			if err != nil {
				log.Println(err)
				state.Errors = append(state.Errors, err)
			}
		}
	}
	return
}

func (state *NodeState) LoadSlots() (err error) {
	rpc_errors := make(chan error, 4)
	var waitgroup sync.WaitGroup
	waitgroup.Add(3)

	go func() {
		defer waitgroup.Done()
		ctx, cancel := context.WithTimeout(context.Background(), RpcTimeout)
		defer cancel()

		var err error
		state.MaxRetransmitSlot, err = state.client.GetMaxRetransmitSlot(ctx)
		if err != nil {
			rpc_errors <- err
		}
	}()

	go func() {
		defer waitgroup.Done()
		ctx, cancel := context.WithTimeout(context.Background(), RpcTimeout)
		defer cancel()

		var err error
		state.CurrentSlot, err = state.client.GetSlot(ctx, solanarpc.CommitmentConfirmed)
		if err != nil {
			rpc_errors <- err
		} else {
		}
	}()

	go func() {
		defer waitgroup.Done()
		ctx, cancel := context.WithTimeout(context.Background(), RpcTimeout)
		defer cancel()

		var err error
		state.ProcessedSlot, err = state.client.GetSlot(ctx, solanarpc.CommitmentProcessed)
		if err != nil {
			rpc_errors <- err
		} else {
		}
	}()

	waitgroup.Wait()
	close(rpc_errors)

	// we log the errors here but we don't cause any fuirther error hanlding
	if len(rpc_errors) > 0 {
		state.HasErrors = true
		for err = range rpc_errors {
			if err != nil {
				log.Println(err)
				state.Errors = append(state.Errors, err)
			}
		}
	}

	return
}

func NewNodeState(node string) *NodeState {
	return &NodeState{
		client:      solanarpc.NewClient(node),
		epochLoaded: false,
		RpcNode:     node,
	}
}
