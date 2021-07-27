package main

import (
	"fmt"
	"log"
	"os"

	solanahc "github.com/linuskendall/solana-rpc-health-check/health-check"
	"github.com/linuskendall/solana-rpc-health-check/rpc"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("usage: ", os.Args[0], " rpcnode1 [rpcnode2 [rpcnode3]]")
	}

	states := solanahc.NewNodeStates(os.Args[1:len(os.Args)], true, true)

	nStates, err := states.LoadStates()
	if err != nil {
		log.Println("error: ", err)
	}

	if nStates < 1 {
		log.Fatal("couldn't load any states")
	}

	currentEpoch := states.States[0].Epoch.Epoch
	previousEpoch := states.States[0].Epoch.Epoch - rpc.Epoch(1)
	epoch_schedule := states.States[0].EpochSchedule

	log.Println("Epoch ", previousEpoch, " first slot ", epoch_schedule.GetFirstSlotInEpoch(previousEpoch), " last slot ", epoch_schedule.GetLastSlotInEpoch(previousEpoch))
	log.Println("Epoch ", currentEpoch, " first slot ", epoch_schedule.GetFirstSlotInEpoch(currentEpoch), " last slot ", epoch_schedule.GetLastSlotInEpoch(currentEpoch))

	fmt.Println("id,rpcNode,minSlot,curSlot,maxRetransmitSlot,slotsStored,prevEpochBlocks,curEpochBlocks")
	for id, state := range states.States {
		fmt.Println(id, ",", state.RpcNode, ",", state.MinimumSlot, ",", state.MaxRetransmitSlot, ",", state.CurrentSlot, ",", uint64(state.CurrentSlot-state.MinimumSlot), ",", len(state.PrevEpochBlocks), ",", len(state.CurEpochBlocks))
	}

	return
}
