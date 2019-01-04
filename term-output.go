package main

import (
	"fmt"
	"strings"
	"time"

	tm "github.com/buger/goterm"
)

var screenTemplate = `
Welcome to Firehose Analyzer - %s

Job                    Instance-Counts     CPU-User     CPU-Sys     CPU-Wait      Memory
----------------------------------------------------------------------------------------
Traffic Controller   %3d                   %5.2f        %5.2f       %5.2f         %5.2f
Doppler              %3d                   %5.2f        %5.2f       %5.2f         %5.2f
Syslog Aadapter      %3d                   %5.2f        %5.2f       %5.2f         %5.2f
Syslog Scheduler     %3d                   %5.2f        %5.2f       %5.2f         %5.2f

Drain Information:	
Syslog Adapter drain bindings  : %.0f
Syslog Scheduler drains        : %.0f
Doppler Sinks Dropped          : %d

Doppler Message Rate Capcity   : ` + tm.Color("%.2f", tm.YELLOW) + `

%s

Metron Health: Report any metron agents that are dropping envelopes
%s


%s
`

var progressBar = ""

func updateTerm() {
	tm.Clear()
	tm.MoveCursor(1, 1)

	tcCount, tcUser, tcSys, tcWait, tcMem := computeInstance(TrafficControllerJob)
	dCount, dUser, dSys, dWait, dMem := computeInstance(DopplerJob)
	saCount, saUser, saSys, saWait, saMem := computeInstance(SyslogAdapterJob)
	ssCount, ssUser, ssSys, ssWait, ssMem := computeInstance(SyslogSchedulerJob)

	envStats := "Doppler\t\t\t\t\t\tSubscriptions\tIngress\tDropped\tLoss\n"
	envStats += "----------------------------------------------------------------------------------------\n"
	dMessSum := uint64(0)
	sinksDroppedSum := uint64(0)
	totalDopplers := 0
	for i := range mc.EnvelopeStats {
		dMessSum += mc.EnvelopeStats[i].Ingress
		totalDopplers++
		sinksDroppedSum += mc.EnvelopeStats[i].SinksDropped
		envStats += fmt.Sprintf("%s/%s\t%.0f\t\t%d\t%d\t%.2f\n", mc.EnvelopeStats[i].Job,
			mc.EnvelopeStats[i].Index,
			mc.EnvelopeStats[i].Subscriptions,
			mc.EnvelopeStats[i].Ingress,
			mc.EnvelopeStats[i].Dropped,
			float64(mc.EnvelopeStats[i].Dropped)/float64(mc.EnvelopeStats[i].Ingress))
	}
	capcity := float64(dMessSum) / float64(totalDopplers)

	metronStats := tm.Color("No unhealthy Metron Agents to report :-)", tm.GREEN)
	for i := range mc.Metrons {
		if mc.Metrons[i].Dropped > 0 {
			metronStats += fmt.Sprintf(tm.Color("%s/%s received %d and dropped %d", tm.RED)+"\n", mc.Metrons[i].Job, mc.Metrons[i].Index, mc.Metrons[i].Ingress, mc.Metrons[i].Dropped)
		}
	}
	tm.Printf(screenTemplate,
		time.Now().Format(time.UnixDate),
		tcCount,
		tcUser,
		tcSys,
		tcWait,
		tcMem,
		dCount,
		dUser,
		dSys,
		dWait,
		dMem,
		saCount,
		saUser,
		saSys,
		saWait,
		saMem,
		ssCount,
		ssUser,
		ssSys,
		ssWait,
		ssMem, mc.AdapterDrainBindings, mc.SchedulerDrains, sinksDroppedSum, capcity, envStats, metronStats, progressBar)
	//tm.Printf("%v\n", mc)
	tm.Flush()
}

func loopTerm() {
	for {
		time.Sleep(5 * time.Second)
		updateTerm()
	}
}

func computeInstance(job string) (count int, user, sys, wait, mem float64) {
	for i := range mc.Instances {
		if mc.Instances[i].Job == job {
			count++
			user += mc.Instances[i].CPUUser
			sys += mc.Instances[i].CPUSys
			wait += mc.Instances[i].CPUWait
			mem += mc.Instances[i].Memory
		}
	}
	user = user / float64(count)
	sys = sys / float64(count)
	wait = wait / float64(count)
	mem = mem / float64(count)
	return
}

// used for replay progress
func updateProgressBar(percent float64) {
	length := 80
	fill := int(float64(length) * percent)
	progressBar = fmt.Sprintf("|%s%s|%3d%%", strings.Repeat("#", fill), strings.Repeat("-", length-fill), int(percent*100))
}
