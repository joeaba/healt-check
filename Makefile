GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
BINDIR=bin

all: static dynamic

dynamic:
	$(GOBUILD) -o $(BINDIR) ./cmd/csv-health-check ./cmd/haproxy-ea-health-check ./cmd/health-check-exporter

static:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) -a -o $(BINDIR)/csv-health-check-static ./cmd/csv-health-check
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) -a -o $(BINDIR)/haproxy-ea-health-check-static ./cmd/haproxy-ea-health-check
