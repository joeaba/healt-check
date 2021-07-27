# solana-rpc-health-check
External HAproxy health check for Solana RPC backend.

# Install go

https://golang.org/doc/install

# Compile

`go build -o bin ./cmd/csv-health-check ./cmd/haproxy-ea-health-check`

# Build static binary

` CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o bin/csv-health-check-static ./cmd/csv-health-check`
` CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o bin/haproxy-ea-health-check-static ./cmd/haproxy-ea-health-check`

# Run CSV health check

`./bin/csv-health-check http://node1.rpc.com http://node2.rpc.com`

# Run as haproxy health check

The `bin/haproxy-ea-health-check` is intended to be run as a server as part of an `agent-check` line on haproxy. In this mode it'll report "up","down #<reason>" or "maint" which will update the haproxy status for a specific server. Currently it's only configured to check on a single server given in `-rpc`, but will eventually be able to report on multiple different servers based on the arguments provided by haproxy.
  
  ```
Usage of ./bin/haproxy-ea-health-check:
  -addr string
        Listen address (default ":9999")
  -block-diff int
        Maximum divergence in blocks (default 300)
  -down int
        Number of consecutive health checks that report down before node is healthy (default 4)
  -enable-block-check
        Enable checking block storage for consecutive blocks (expensive)
  -enable-max-retransmit-check
        Enable checking max retransmit slots (default true)
  -maintfile string
        A file which if exists puts this server in maintenance mode (default "/etc/haproxy/maintenance")
  -minimum-ledger-size int
        Minimum number of slots that node needs to have stored
  -reference-servers string
        Enables checking the current slot against provided comma separated list of reference servers
  -rpc string
        Solana RPC URI (including protocol and path) (default "http://localhost:8899")
  -rpc-timeout int
        Timeout per rpc call (default 10)
  -slot-diff int
        Maximum divergence in slots (default 200)
  -up int
        Number of consecutive health checks that report up before node is healthy (default 2)
```

# Sample service file

```
[Unit]
Description=HAProxy Load Balancer Health Check
After=network-online.target
Wants=network-online.target

[Service]
ExecStart=/usr/local/bin/haproxy-ea-health-check -rpc "http://127.0.0.1:8899" -reference-servers="http://api.rpcpool.com,https://solana-api.projectserum.com" -enable-max-retransmit-check=false -enable-block-check=false -minimum-ledger-size=0
Restart=always
Type=simple

[Install]
WantedBy=multi-user.target
```
# Sample haproxy config line

```
server localhost 127.0.0.1:8899 maxconn 600 check  agent-check agent-inter 10s agent-addr 127.0.0.1 agent-port 9999
```

# Maintenance mode

The server can be put into maintenance mode by touching the maintfile (e.g. "/etc/haproxy/maintenance"). Deleting this file will allow the server to come back up again.
