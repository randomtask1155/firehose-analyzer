package main

import (
	"fmt"
	"time"

	tm "github.com/buger/goterm"
)

var screenTemplate = `
Welcome to Firehose Analyzer - %s

Job                    Instance-Counts     CPU-User     CPU-Sys     CPU-Wait      Memory
----------------------------------------------------------------------------------------
Traffic Controller     %d                   %.2f         %.2f        %.2f          %.2f
Doppler                %d                   %.2f         %.2f        %.2f          %.2f
Syslog Aadapter        %d                   %.2f         %.2f        %.2f          %.2f
Syslog Scheduler       %d                   %.2f         %.2f        %.2f          %.2f

Drain Information:	
Syslog Adapter		- There are %.0f drain bindings
Syslog Scheduler	- There are %.0f drains
	
%s

Metron Health: Report any metron agents that are dropping envelopes
%s
`

func updateTerm() {
	tm.Clear()
	tm.MoveCursor(1, 1)

	tcCount, tcUser, tcSys, tcWait, tcMem := computeInstance(TrafficControllerJob)
	dCount, dUser, dSys, dWait, dMem := computeInstance(DopplerJob)
	saCount, saUser, saSys, saWait, saMem := computeInstance(SyslogAdapterJob)
	ssCount, ssUser, ssSys, ssWait, ssMem := computeInstance(SyslogSchedulerJob)

	envStats := "Doppler\t\t\t\t\t\tSubscriptions\tIngress\t\tDropped\n"
	envStats += "----------------------------------------------------------------------------------------\n"
	for i := range mc.EnvelopeStats {
		envStats += fmt.Sprintf("%s/%s\t%.0f\t\t%d\t\t%d\n", mc.EnvelopeStats[i].Job, mc.EnvelopeStats[i].Index, mc.EnvelopeStats[i].Subscriptions, mc.EnvelopeStats[i].Ingress, mc.EnvelopeStats[i].Dropped)
	}

	metronStats := "No unhealthy Metron Agents to report :-)"
	for i := range mc.Metrons {
		if mc.Metrons[i].Dropped > 0 {
			metronStats += fmt.Sprintf("%s/%s received %d and dropped %d\n", mc.Metrons[i].Job, mc.Metrons[i].Index, mc.Metrons[i].Ingress, mc.Metrons[i].Dropped)
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
		ssMem, mc.AdapterDrainBindings, mc.SchedulerDrains, envStats, metronStats)
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
