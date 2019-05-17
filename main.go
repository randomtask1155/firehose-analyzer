package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/cloudfoundry/dropsonde/dropsonde_unmarshaller"
	"github.com/cloudfoundry/sonde-go/events"
)

var (
	systemDomain   = flag.String("sys", "", "Provide the system domain name which is used to find the log-cache endpoint")
	accessToken    = flag.String("token", "", "Provide an access token used to authenticate with doppler endpoint. Defaults to ~/.cf/config.json")
	outFile        = flag.String("o", "", "Specifiy an output file that records data in csv format")
	replay         = flag.String("replay", "", "-replay [filename]\nReplay stats from given output file.  Also see -speed to adjust replay settings")
	speed          = flag.Int("speed", 1, "Speed of replay.  Default 1 is realtime and 0 for instance replay")
	logger         *log.Logger
	mc             Metrics
	arvhiveEnabled = false
	ofh            *os.File
	lcc            LCC
)

const (
	tcJob              = "loggregator_trafficcontroller"
	dopplerJob         = "doppler"
	syslogAdapterJob   = "syslog_adapter"
	syslogSchedulerJob = "syslog_scheduler"

	trafficControllerSID   = "traffic_controller"
	dopplerSID             = "doppler"
	syslogDrainAdapterSID  = "drain_adapter"
	syslogDrainScheduleSID = "drain_scheduler"
	metronSID              = "metron"
	logCacheSID            = "log-cache"
	logCacheNozzleSID      = "log-cache-nozzle"
	boshSystemMetricsSID   = "bosh-system-metrics-forwarder" // cpu and memory

	// common stats
	ingressCounter = "ingress"
	egressCounter  = "egress"
	droppedCounter = "dropped"

	// boshSystemMetricsSID metrics
	cpuUserGauge       = "system_cpu_user"
	cpuWaitGauge       = "system_cpu_wait"
	cpuSYSGauge        = "system_cpu_sys"
	memoryPercentGauge = "system_mem_percent"

	// trafficControllerSID metrics
	slowConsumerCounter          = "doppler_proxy_slow_consumer"
	firehosesGauge               = "doppler_proxy_firehoses"
	appStreamsGauge              = "doppler_proxy_app_streams"
	containerMetricsLatencyGauge = "doppler_proxy_container_metrics_latency"

	// dopplerSID metrics
	dopplerSubscritionsGauge = "subscriptions"
	dumpSinksGauge           = "dump_sinks"
	sinksDroppedCounter      = "sinks_dropped"
	sinkErrorsDroppedCounter = "sinks_errors_dropped"

	// syslogDrainAdapterSID metrics
	drainBindingsGauge = "drain_bindings"

	// syslogDrainScheduleSID
	drainsGauge   = "drains"
	adaptersGauge = "adpaters"

	// metronSID
	averageEnvelopGauge = "average_envelope" // bytes/minute

	// logCacheSID
	lcSystemMemGauge   = "available-system-memory"
	lcTotalMemGauge    = "total-system-memory"
	lcExpiredCounter   = "expired"
	lcCachePeriodGauge = "cache-period"

	// logCacheNozzleSID
	lcnErrCounter = "err"
)

func main() {
	logger = log.New(os.Stdout, "logger: ", log.Ldate|log.Ltime|log.Lshortfile)
	flag.Parse()

	if *systemDomain == "" {
		logger.Fatalln("Must provide system domain with \"-sys domain.com\"")
	}

	mc = Metrics{}
	if *replay != "" {
		//	go runReplay()
		//	loopTerm()
		return
	}

	var err error
	lcc, err = NewLogCacheClient(fmt.Sprintf("https://log-cache.%s", *systemDomain))
	if err != nil {
		logger.Fatalf("Could not create log cache client: %s\n", err)
	}
	logger.Fatalln(lcc.Collect())

	input := make(chan []byte, 5000)
	output := make(chan *events.Envelope, 10000)
	dn := dropsonde_unmarshaller.NewDropsondeUnmarshaller()

	if *outFile != "" {
		logger.Printf("starting to archive output to %s", *outFile)
		var err error
		ofh, err = os.Create(*outFile)
		if err != nil {
			logger.Fatalln(err)
		}
		ofh.Write([]byte(fmt.Sprintf("time,origin,job/index,metric,value,type,unit\n")))
		arvhiveEnabled = true
		defer ofh.Close()
	}

	logger.Println("starting dropsnode unmarshaller...")
	go dn.Run(input, output)

	logger.Println("starting output collector...")
	/*go func(output chan *events.Envelope) {
		for {
			select {
			case e := <-output:
				mc.parseEnvelope(e)
			}
		}
	}(output)*/

	logger.Println("starting read loop...")
	//go loopTerm()
	/*for {
		_, p, err := conn.ReadMessage()
		if err != nil {
			logger.Println(err)
			time.Sleep(1 * time.Second)
		}
		input <- p
	}*/

}
