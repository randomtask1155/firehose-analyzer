package main

import (
	"fmt"
	"strings"
	"time"

	tm "github.com/buger/goterm"
)

var screenTemplate = `
Welcome to Firehose Analyzer - %s
Selected duration=%s and offset=%s

Job                    Instance-Counts     CPU-User     CPU-Sys     CPU-Wait      Memory
----------------------------------------------------------------------------------------
Traffic Controller   %3d                   %5.2f        %5.2f       %5.2f         %5.2f
Doppler              %3d                   %5.2f        %5.2f       %5.2f         %5.2f
Syslog Aadapter      %3d                   %5.2f        %5.2f       %5.2f         %5.2f
Syslog Scheduler     %3d                   %5.2f        %5.2f       %5.2f         %5.2f

Drain Information:
Syslog Adapter drain bindings  : %.0f
Syslog Scheduler drains        : %.0f
Doppler Sinks Dropped          : %.0f

Doppler Ingress Max Dropped    : %.0f
Doppler Message Rate Capcity   : ` + tm.Color("%.2f", tm.YELLOW) + `

%s


%s
`

var progressBar = ""

func updateTerm(lcc *LCC) {
	lcc.Lock()
	defer lcc.Unlock()

	tm.Clear()
	tm.MoveCursor(1, 1)

	envStats := "Job\t\t\t\tSubscriptions\tSum-Ingress/s\tDropped/s\tLoss\n"
	envStats += "----------------------------------------------------------------------------------------\n"
	envStats += fmt.Sprintf("Doppler\t\t\t\t%.0f\t\t%.0f\t\t%.0f\t\t%.2f\n", lcc.Metric.Doppler.Subscriptions,
		lcc.Metric.Doppler.Ingress,
		lcc.Metric.Doppler.Dropped,
		float64(lcc.Metric.Doppler.Dropped)/float64(lcc.Metric.Doppler.Ingress))
	envStats += fmt.Sprintf("Metron\t\t\t\t%d\t\t%.0f\t\t%.0f\t\t%.2f\n", 0,
		lcc.Metric.Metron.Ingress,
		lcc.Metric.Metron.Dropped,
		float64(lcc.Metric.Metron.Dropped)/float64(lcc.Metric.Metron.Ingress))
	envStats += fmt.Sprintf("Reverse Log Proxy\t\t%d\t\t%.0f\t\t%.0f\t\t%.2f\n", 0,
		lcc.Metric.RLP.Ingress,
		lcc.Metric.RLP.Dropped,
		float64(lcc.Metric.RLP.Dropped)/float64(lcc.Metric.RLP.Ingress))

	tm.Printf(screenTemplate,
		time.Now().Format(time.UnixDate),
		*sampleDuration,
		*sampleOffset,
		lcc.Metric.TC.System.Count,
		lcc.Metric.TC.System.CPUUser,
		lcc.Metric.TC.System.CPUSys,
		lcc.Metric.TC.System.CPUWait,
		lcc.Metric.TC.System.Memory,
		lcc.Metric.Doppler.System.Count,
		lcc.Metric.Doppler.System.CPUUser,
		lcc.Metric.Doppler.System.CPUSys,
		lcc.Metric.Doppler.System.CPUWait,
		lcc.Metric.Doppler.System.Memory,
		lcc.Metric.SyslogAdapter.System.Count,
		lcc.Metric.SyslogAdapter.System.CPUUser,
		lcc.Metric.SyslogAdapter.System.CPUSys,
		lcc.Metric.SyslogAdapter.System.CPUWait,
		lcc.Metric.SyslogAdapter.System.Memory,
		lcc.Metric.SyslogScheduler.System.Count,
		lcc.Metric.SyslogScheduler.System.CPUUser,
		lcc.Metric.SyslogScheduler.System.CPUSys,
		lcc.Metric.SyslogScheduler.System.CPUWait,
		lcc.Metric.SyslogScheduler.System.Memory,
		lcc.Metric.Drain.DrainBindings,
		lcc.Metric.Drain.ScheduledDrains,
		lcc.Metric.Drain.SinksDropped,
		lcc.Metric.Doppler.IngressDropped,
		lcc.Metric.Doppler.MessageRateCapacity,
		envStats,
		progressBar)
	//tm.Printf("%v\n", mc)
	tm.Flush()
}

func loopTerm(lcc *LCC) {
	for {
		time.Sleep(5 * time.Second)
		updateTerm(lcc)
	}
}

// used for replay progress
func updateProgressBar(percent float64) {
	length := 80
	fill := int(float64(length) * percent)
	progressBar = fmt.Sprintf("|%s%s|%3d%%", strings.Repeat("#", fill), strings.Repeat("-", length-fill), int(percent*100))
}
