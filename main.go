package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"code.cloudfoundry.org/cli/plugin"
)

var (
	logger         *log.Logger
	mc             Metrics
	arvhiveEnabled = false
	ofh            *os.File
	accessToken    string
	systemDomain   string
	cfCLI          plugin.CliConnection
)

// BasicPlugin implement cf cli plugin api
type BasicPlugin struct{}

// Run execute the firehose analyzer tool
func (c *BasicPlugin) Run(cliConnection plugin.CliConnection, args []string) {
	// Ensure that we called the command basic-plugin-command
	cfCLI = cliConnection
	if args[0] == "firehose-analyzer" {
		startAnalyzer()
	}
}

// GetMetadata interface for plugin api
func (c *BasicPlugin) GetMetadata() plugin.PluginMetadata {
	return plugin.PluginMetadata{
		Name: "firehose-analyzer",
		Version: plugin.VersionType{
			Major: 1,
			Minor: 1,
			Build: 1,
		},
		MinCliVersion: plugin.VersionType{
			Major: 6,
			Minor: 7,
			Build: 0,
		},
		Commands: []plugin.Command{
			{
				Name:     "firehose-analyzer",
				HelpText: "Displays basic firehose metrics for troubleshooting scaling issues",

				// UsageDetails is optional
				// It is used to show help of usage of each command
				UsageDetails: plugin.Usage{
					Usage: "firehose-analyzer\n   cf firehose-analyzer",
				},
			},
		},
	}
}

const (
	tcJob              = "loggregator_trafficcontroller"
	dopplerJob         = "doppler"
	syslogAdapterJob   = "syslog_adapter"
	syslogSchedulerJob = "syslog_scheduler"
	metronJob          = "metron"

	trafficControllerSID   = "traffic_controller"
	dopplerSID             = "doppler"
	syslogDrainAdapterSID  = "drain_adapter"
	syslogDrainScheduleSID = "drain_scheduler"
	metronSID              = "metron"
	logCacheSID            = "log-cache"
	logCacheNozzleSID      = "log-cache-nozzle"
	rlpSID                 = "reverse_log_proxy"
	boshSystemMetricsSID   = "bosh-system-metrics-forwarder" // cpu and memory

	// common stats
	ingressCounter     = "ingress"
	egressCounter      = "egress"
	droppedCounter     = "dropped"
	subscriptionsGauge = "subscriptions"

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

func startAnalyzer() {
	mc = Metrics{}
	apiURL, err := cfCLI.ApiEndpoint()
	if err != nil {
		logger.Fatalln(err)
	}
	sysDomain := fmt.Sprintf("https://log-cache.%s", apiURL[12:len(apiURL)])
	lcc, err := NewLogCacheClient(sysDomain)
	if err != nil {
		logger.Fatalf("Could not create log cache client: %s\n", err)
	}
	go loopTerm(&lcc)
	for {
		lcc.Collect()
		time.Sleep(30 * time.Second)
	}
}

func main() {
	logger = log.New(os.Stdout, "logger: ", log.Ldate|log.Ltime|log.Lshortfile)
	plugin.Start(new(BasicPlugin))
}
