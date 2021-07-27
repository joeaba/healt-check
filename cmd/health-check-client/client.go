package main

import (
  "log"
  "net/rpc"
  solRpc "github.com/linuskendall/solana-rpc-health-check/rpc"
)

type Args struct {
}


type State struct {
  MinimumSlot solRpc.Slot
  CurrentSlot solRpc.Slot
  PrevEpochBlocks uint64
  CurEpochBlocks uint64
}


func main() {
  client, err := rpc.DialHTTP("tcp", "127.0.0.1:9990")
  if err != nil {
    log.Fatal("dialing:", err)
  }

  var state State
  var args Args
  err = client.Call("Health.GetState", &args, &state)
  if err != nil {
    log.Fatal("health error:", err)
  }

  log.Printf("slots=%d", state.MinimumSlot)
}
