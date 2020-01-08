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

Drain Information:
Syslog Agent drain bindings     : %.0f
Syslog Agent Active Drains      : %.0f
Syslog Agent Invalid Drains     : %.0f
Syslog Agent Non-App Drains     : %.0f
Syslog Agent Blacklisted Drains : %.0f

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

	envStats := "Job\t\tSubscriptions\tIngress/s\tEgress/s\tDropped/s\tLoss\n"
	envStats += "----------------------------------------------------------------------------------------\n"
	envStats += fmt.Sprintf("Doppler\t\t%.0f\t\t%.0f\t\t%.0f\t\t%.0f\t\t%.2f\n", lcc.Metric.Doppler.Subscriptions,
		lcc.Metric.Doppler.Ingress,
		lcc.Metric.Doppler.Egress,
		lcc.Metric.Doppler.Dropped,
		float64(lcc.Metric.Doppler.Dropped)/float64(lcc.Metric.Doppler.Ingress))
	envStats += fmt.Sprintf("Metron\t\tN/A\t\t%.0f\t\t%.0f\t\t%.0f\t\t%.2f\n", lcc.Metric.Metron.Ingress,
		lcc.Metric.Metron.Egress,
		lcc.Metric.Metron.Dropped,
		float64(lcc.Metric.Metron.Dropped)/float64(lcc.Metric.Metron.Ingress))
	envStats += fmt.Sprintf("RLP\t\tN/A\t\t%.0f\t\t%.0f\t\t%.0f\t\t%.2f\n", lcc.Metric.RLP.Ingress,
		lcc.Metric.RLP.Egress,
		lcc.Metric.RLP.Dropped,
		float64(lcc.Metric.RLP.Dropped)/float64(lcc.Metric.RLP.Ingress))
	envStats += fmt.Sprintf("Syslog Agent\tN/A\t\t%.0f\t\t%.0f\t\t%.0f\t\t%.2f\n", lcc.Metric.Drain.AgentIngress,
		lcc.Metric.Drain.AgentEgress,
		lcc.Metric.Drain.AgentDropped,
		float64(lcc.Metric.Drain.AgentDropped)/float64(lcc.Metric.Drain.AgentIngress)) // syslog agent loss rate

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
		lcc.Metric.Drain.AgentBindings,
		lcc.Metric.Drain.AgentActiveDrains,
		lcc.Metric.Drain.AgentInvalidDrains,
		lcc.Metric.Drain.AgentNonAppDrains,
		lcc.Metric.Drain.AgentBlacklistedDrains,
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
