package main

import (
	"flag"
	"log"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/firstrow/tcp_server"
)

const (
	TIMEOUT                time.Duration = 10 * time.Second
	HEALTH_UPDATE_INTERVAL time.Duration = 10 * time.Second
)

var (
	rpcURI                     = flag.String("rpc", "http://localhost:8899", "Solana RPC URI (including protocol and path)")
	addr                       = flag.String("addr", ":9999", "Listen address")
	rpcTimeout                 = flag.Int("rpc-timeout", 10, "Timeout per rpc call")
	maintPath                  = flag.String("maintfile", "/etc/haproxy/maintenance", "A file which if exists puts this server in maintenance mode")
	MAX_SLOT_DIFF              = flag.Int("slot-diff", 200, "Maximum divergence in slots")
	MAX_BLOCK_DIFF             = flag.Int("block-diff", 300, "Maximum divergence in blocks")
	UP_THRESHOLD               = flag.Int("up", 2, "Number of consecutive health checks that report up before node is healthy")
	DOWN_THRESHOLD             = flag.Int("down", 4, "Number of consecutive health checks that report down before node is healthy")
	BLOCK_CHECK_ENABLED        = flag.Bool("enable-block-check", false, "Enable checking block storage for consecutive blocks (expensive)")
	MAX_TRANSMIT_CHECK_ENABLED = flag.Bool("enable-max-retransmit-check", true, "Enable checking max retransmit slots")
	MINIMUM_LEDGER_SIZE        = flag.Int("minimum-ledger-size", 0, "Minimum number of slots that node needs to have stored")
	REFERENCE_SERVERS          = flag.String("reference-servers", "", "Enables checking the current slot against provided comma separated list of reference servers")
)

func main() {
	flag.Parse()

	var servers []string
	if *REFERENCE_SERVERS != "" {
		servers = strings.Split(*REFERENCE_SERVERS, ",")
		for i := 0; i < len(servers); i++ {
			servers[i] = strings.TrimSpace(servers[i])
		}
	}

	log.Println("Listening on ", *addr, "testing rpc", *rpcURI)

	var one_check_enabled bool = false

	log.Println("Checks: ")
	if len(servers) > 0 {
		one_check_enabled = true
		log.Println("+ Reference server comparison check: slots=", *MAX_SLOT_DIFF, "servers=", servers)
	} else {
		log.Println("- Reference server comparison check disabled.")
	}

	if *MAX_TRANSMIT_CHECK_ENABLED {
		one_check_enabled = true
		log.Println("+ Max transmit check: ", *MAX_SLOT_DIFF)
	} else {
		log.Println("- Max transmit check disabled..")
	}

	if *BLOCK_CHECK_ENABLED {
		one_check_enabled = true
		log.Println("+ Block check: ", *MAX_BLOCK_DIFF)
	} else {
		log.Println("- Block check disabled.")
	}

	if *MINIMUM_LEDGER_SIZE > 0 {
		one_check_enabled = true
		log.Println("+ Ledger size requirement: ", *MINIMUM_LEDGER_SIZE)
	} else {
		log.Println("- Ledger size requirement disabled.")
	}

	if !one_check_enabled {
		log.Println("WARNING: All checks are disabled. This will always return up.")
	}

	// Load initial state
	health_state := NewHealthState(*rpcURI, servers)
	go health_state.UpdateState(HEALTH_UPDATE_INTERVAL)

	server := tcp_server.New(*addr)

	var maintenance_mode uint32 = 0

	server.OnNewClient(func(c *tcp_server.Client) {
		defer c.Close()
		// Set the server to maintenance mode
		if _, err := os.Stat(*maintPath); err == nil {
			atomic.StoreUint32(&maintenance_mode, 1)
			c.Send("maint\n")
			return
		} else if atomic.LoadUint32(&maintenance_mode) == 1 {
			atomic.StoreUint32(&maintenance_mode, 0)
			c.Send("ready\n")
			return
		}

		status := health_state.GetStatus()
		log.Println("answering health request node is: ", status)
		c.Send(status + "\n")

		return
	})

	server.Listen()
}
