package main

//"context"
//"os"
//"fmt"
import (
	"flag"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	solanahc "github.com/linuskendall/solana-rpc-health-check/health-check"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	httpTimeout = 5 * time.Second
)

type Exporter struct {
	rpcURI              string
	poolDesc            *prometheus.Desc
	infoDesc            *prometheus.Desc
	currentSlotDesc     *prometheus.Desc
	processedSlotDesc   *prometheus.Desc
	minimumSlotDesc     *prometheus.Desc
	slotsStoredDesc     *prometheus.Desc
	prevEpochBlocksDesc *prometheus.Desc
	curEpochBlocksDesc  *prometheus.Desc
}

func NewExporter(uri string) *Exporter {
	return &Exporter{
		rpcURI: uri,
		poolDesc: prometheus.NewDesc(
			"rpcpool_info",
			"Information about the rpcpool",
			[]string{"rpc", "pool", "region"}, nil),
		infoDesc: prometheus.NewDesc(
			"solana_info",
			"Information about the validator process",
			[]string{"rpc", "version", "feature_set", "identity", "genesis_hash"}, nil),
		currentSlotDesc: prometheus.NewDesc(
			"solana_current_slot",
			"The current slot stored by RPC server",
			[]string{"rpc"}, nil),
		processedSlotDesc: prometheus.NewDesc(
			"solana_processed_slot",
			"The processed slot stored by RPC server",
			[]string{"rpc"}, nil),
		minimumSlotDesc: prometheus.NewDesc(
			"solana_minimum_slot",
			"The minimum slot stored by RPC server",
			[]string{"rpc"}, nil),
		slotsStoredDesc: prometheus.NewDesc(
			"solana_slots_stored",
			"The number of slots stored by RPC server",
			[]string{"rpc"}, nil),
		prevEpochBlocksDesc: prometheus.NewDesc(
			"solana_previous_epoch_blocks",
			"The number of blocks from previous epoch stored by RPC server",
			[]string{"rpc"}, nil),
		curEpochBlocksDesc: prometheus.NewDesc(
			"solana_current_epoch_blocks",
			"The number of blocks from current epoch stored by RPC server",
			[]string{"rpc"}, nil),
	}
}

func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- e.poolDesc
	ch <- e.infoDesc
	ch <- e.currentSlotDesc
	ch <- e.processedSlotDesc
	ch <- e.minimumSlotDesc
	ch <- e.slotsStoredDesc
	ch <- e.prevEpochBlocksDesc
	ch <- e.curEpochBlocksDesc
}

func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	nodeState := solanahc.NewNodeState(e.rpcURI)
	err := nodeState.LoadEpoch()

	err = nodeState.LoadMinimumLedger()
	if err != nil {
		ch <- prometheus.NewInvalidMetric(e.minimumSlotDesc, err)
	} else {
		ch <- prometheus.MustNewConstMetric(e.minimumSlotDesc, prometheus.GaugeValue, float64(nodeState.MinimumSlot), e.rpcURI)
	}

	err = nodeState.LoadBlocks()
	if err != nil {
		//	ch <- prometheus.NewInvalidMetric(e.prevEpochBlocksDesc, nodeState.Errors[0])
		//	ch <- prometheus.NewInvalidMetric(e.curEpochBlocksDesc, nodeState.Errors[0])
	} else {
		ch <- prometheus.MustNewConstMetric(e.prevEpochBlocksDesc, prometheus.GaugeValue, float64(len(nodeState.PrevEpochBlocks)), e.rpcURI)
		ch <- prometheus.MustNewConstMetric(e.curEpochBlocksDesc, prometheus.GaugeValue, float64(len(nodeState.CurEpochBlocks)), e.rpcURI)
	}

	err = nodeState.LoadSlots()
	if err != nil {
		ch <- prometheus.NewInvalidMetric(e.currentSlotDesc, nodeState.Errors[0])
		ch <- prometheus.NewInvalidMetric(e.processedSlotDesc, nodeState.Errors[0])
		ch <- prometheus.NewInvalidMetric(e.slotsStoredDesc, nodeState.Errors[0])
	} else {
		ch <- prometheus.MustNewConstMetric(e.currentSlotDesc, prometheus.GaugeValue, float64(nodeState.CurrentSlot), e.rpcURI)
		ch <- prometheus.MustNewConstMetric(e.processedSlotDesc, prometheus.GaugeValue, float64(nodeState.ProcessedSlot), e.rpcURI)
		ch <- prometheus.MustNewConstMetric(e.slotsStoredDesc, prometheus.GaugeValue, float64(nodeState.CurrentSlot-nodeState.MinimumSlot), e.rpcURI)
	}

	err = nodeState.LoadMeta()
	if err != nil {
		ch <- prometheus.NewInvalidMetric(e.infoDesc, nodeState.Errors[0])
	} else {
		ch <- prometheus.MustNewConstMetric(e.infoDesc, prometheus.GaugeValue, float64(1), e.rpcURI, nodeState.Version.CoreVersion, strconv.Itoa(int(nodeState.Version.FeatureSet)), nodeState.Identity.Identity, nodeState.GenesisHash)
	}

	mutex.Lock()
	ch <- prometheus.MustNewConstMetric(e.poolDesc, prometheus.GaugeValue, float64(1), e.rpcURI, *poolName, *region)
	mutex.Unlock()
}

var (
	rpcAddr  = flag.String("rpcURI", "", "Solana RPC URI (including protocol and path)")
	addr     = flag.String("addr", ":8080", "Listen address")
	poolFile = flag.String("poolfile", "/etc/haproxy/rpcpool.cfg", "The file to read the pool name from")
	poolName = flag.String("pool", "rpcpool", "default pool name in case poolfile is missing")
	region   = flag.String("region", "", "region name")
	mutex    = &sync.Mutex{}
)

func readPoolName(poolFile string) {
	new_pool_name, err := ioutil.ReadFile(poolFile)
	if err != nil {
		log.Println("readfile error", err)
	} else {
		if len(new_pool_name) > 0 {
			*poolName = strings.TrimSpace(string(new_pool_name))
		}
	}
}

func main() {
	flag.Parse()

	if *rpcAddr == "" {
		log.Fatal("Please specify -rpcURI")
	}

	// creates a new file watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Println("ERROR", err)
	}
	defer watcher.Close()

	// Watch file
	go func() {
		for {
			select {
			// watch for events
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}

				log.Println("event: ", event)
				if event.Op&fsnotify.Create == fsnotify.Create ||
					event.Op&fsnotify.Write == fsnotify.Write ||
					event.Op&fsnotify.Rename == fsnotify.Rename {
					log.Println("modified, renamed or created file: ", event.Name)
					readPoolName(event.Name)
				} else if event.Op&fsnotify.Remove == fsnotify.Remove {
					log.Println("warning, pool file was removed, to update pool name you would need to restart this software")
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Println("watcher error", err)
			}
		}
	}()

	// Create the poolfile if it doesn't exist yet, otherwise read its contents
	_, err = os.Stat(*poolFile)
	if os.IsNotExist(err) {
		file, err := os.Create(*poolFile)
		if err != nil {
			log.Fatal(err)
		}
		file.WriteString(*poolName)
		file.Close()
	} else {
		readPoolName(*poolFile)
	}

	err = watcher.Add(*poolFile)
	if err != nil {
		log.Println("watcher error", err)
	}

	exporter := NewExporter(*rpcAddr)
	prometheus.MustRegister(exporter)

	http.Handle("/metrics", promhttp.Handler())
	log.Fatal(http.ListenAndServe(*addr, nil))
}
