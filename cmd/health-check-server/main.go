package main

import (
	"flag"
	"log"
	"net"
	"net/http"
	"net/rpc"

	solanahc "github.com/linuskendall/solana-rpc-health-check/health-check"
	solanarpc "github.com/linuskendall/solana-rpc-health-check/rpc"
)

var (
	rpcURI     = flag.String("rpc", "http://localhost:8899", "Solana RPC URI (including protocol and path)")
	addr       = flag.String("addr", ":9990", "Listen address")
	rpcTimeout = flag.Int("rpc-timeout", 10, "Timeout per rpc call")
)

type Args struct {
}

type State struct {
	MinimumSlot     solanarpc.Slot
	CurrentSlot     solanarpc.Slot
	PrevEpochBlocks uint64
	CurEpochBlocks  uint64
}

type Health int

func (t *Health) GetState(args *Args, reply *State) error {
	var state State
	nodestate := solanahc.NewNodeState(*rpcURI)
	nodestate.LoadSlots()

	state.MinimumSlot = 100
	state.CurrentSlot = 100
	state.PrevEpochBlocks = 100
	state.CurEpochBlocks = 100

	*reply = state
	return nil
}

func main() {
	flag.Parse()

	health := new(Health)
	err := rpc.Register(health)
	if err != nil {
		log.Fatal("Format of service isn't correct. ", err)
	}
	rpc.HandleHTTP()
	listener, e := net.Listen("tcp", *addr)
	if e != nil {
		log.Fatal("Listen error:", e)
	}

	log.Printf("listening on %s", *addr)

	err = http.Serve(listener, nil)
	if err != nil {
		log.Fatal("serve error: ", err)
	}
}
