package main

import (
	"github.com/cloudfoundry/sonde-go/events"
)

// Metrics container for all metrics
type Metrics struct {
	Instances            []Instance
	AdapterDrainBindings float64
	SchedulerDrains      float64
	EnvelopeStats        []EnvelopeStat
	Metrons              []Metron
}

// Instance cpu and memory stats for a given instance
type Instance struct {
	Job     string
	Index   string
	CPUUser float64
	CPUWait float64
	CPUSys  float64
	Memory  float64
}

// EnvelopeStat ingress and dropped metrics for doppler
type EnvelopeStat struct {
	Job           string
	Index         string
	Subscriptions float64
	Ingress       uint64
	Dropped       uint64
}

/*
origin:"MetronAgent" eventType:CounterEvent timestamp:1543613383423032434 deployment:"service-instance_ff5cb75b-2e90-4ed7-a3e5-2e39582dabe0" job:"mysql" index:"81df1815-0ea1-4393-ad43-2d26e83d6fd1" ip:"10.193.76.48" counterEvent:<name:"dropsondeMarshaller.sentEnvelopes" delta:160 total:6664530 > 17:"\n\nevent_type\x12\vValueMetric" 17:"\n\bprotocol\x12\x04grpc"
origin:"MetronAgent" eventType:CounterEvent timestamp:1543613383422952033 deployment:"service-instance_ff5cb75b-2e90-4ed7-a3e5-2e39582dabe0" job:"mysql" index:"81df1815-0ea1-4393-ad43-2d26e83d6fd1" ip:"10.193.76.48" counterEvent:<name:"dropsondeMarshaller.sentEnvelopes" delta:10 total:416605 > 17:"\n\nevent_type\x12\fCounterEvent" 17:"\n\bprotocol\x12\x04grpc"
origin:"MetronAgent" eventType:CounterEvent timestamp:1543613383423119790 deployment:"service-instance_ff5cb75b-2e90-4ed7-a3e5-2e39582dabe0" job:"mysql" index:"81df1815-0ea1-4393-ad43-2d26e83d6fd1" ip:"10.193.76.48" counterEvent:<name:"dropsondeAgentListener.receivedByteCount" delta:11142 total:464211698 >
origin:"MetronAgent" eventType:CounterEvent timestamp:1543613383423208077 deployment:"service-instance_ff5cb75b-2e90-4ed7-a3e5-2e39582dabe0" job:"mysql" index:"81df1815-0ea1-4393-ad43-2d26e83d6fd1" ip:"10.193.76.48" counterEvent:<name:"dropsondeUnmarshaller.receivedEnvelopes" delta:152 total:6332890 > 17:"\n\bprotocol\x12\x03udp" 17:"\n\nevent_type\x12\vValueMetric"
*/
// Metron stats related to metron sent/received envelopes.   look for agents that are receiving more than 8k (80% of the max of 10k)
type Metron struct {
	Job     string
	Index   string
	Ingress uint64
	Dropped uint64
}

func (m *Metrics) processValueMetric(e *events.Envelope) {
	index := e.GetIndex()
	instanceIndex := -1
	for i := range m.Instances {
		if m.Instances[i].Index == index {
			instanceIndex = i
			break
		}
	}

	if *e.ValueMetric.Name == "system.cpu.wait" && e.GetOrigin() == "bosh-system-metrics-forwarder" {
		m.Instances[instanceIndex].CPUWait = *e.ValueMetric.Value
		return
	}
	if *e.ValueMetric.Name == "system.cpu.user" && e.GetOrigin() == "bosh-system-metrics-forwarder" {
		m.Instances[instanceIndex].CPUUser = *e.ValueMetric.Value
		return
	}
	if *e.ValueMetric.Name == "system.cpu.sys" && e.GetOrigin() == "bosh-system-metrics-forwarder" {
		m.Instances[instanceIndex].CPUSys = *e.ValueMetric.Value
		return
	}
	if *e.ValueMetric.Name == "system.mem.percent" && e.GetOrigin() == "bosh-system-metrics-forwarder" {
		m.Instances[instanceIndex].Memory = *e.ValueMetric.Value
		return
	}

	if *e.ValueMetric.Name == "drains" && e.GetOrigin() == "cf-syslog-drain.scheduler" {
		m.SchedulerDrains = *e.ValueMetric.Value
		return
	}

	if *e.ValueMetric.Name == "drain_bindings" && e.GetOrigin() == "cf-syslog-drain.adapter" {
		m.AdapterDrainBindings = *e.ValueMetric.Value
		return
	}

	// Doppler Metrics
	dopplerIndex := -1
	for i := range m.EnvelopeStats {
		if m.EnvelopeStats[i].Index == index {
			dopplerIndex = i
			break
		}
	}
	if *e.ValueMetric.Name == "subscriptions" && e.GetOrigin() == "loggregator.doppler" {
		m.EnvelopeStats[dopplerIndex].Subscriptions = *e.ValueMetric.Value
		return
	}
}

func (m *Metrics) processCounterEvent(e *events.Envelope) {
	index := e.GetIndex()
	dopplerIndex := -1
	for i := range m.EnvelopeStats {
		if m.EnvelopeStats[i].Index == index {
			dopplerIndex = i
			break
		}
	}
	metronIndex := -1
	for i := range m.Metrons {
		if m.Metrons[i].Index == index {
			metronIndex = i
		}
	}

	if *e.CounterEvent.Name == "ingress" && e.GetOrigin() == "loggregator.doppler" {
		m.EnvelopeStats[dopplerIndex].Ingress = *e.CounterEvent.Delta
		return
	}
	if *e.CounterEvent.Name == "dropped" && e.GetOrigin() == "loggregator.doppler" {
		m.EnvelopeStats[dopplerIndex].Dropped = *e.CounterEvent.Delta
		return
	}

	if *e.CounterEvent.Name == "ingress" && e.GetOrigin() == MetronOrigin {
		m.Metrons[metronIndex].Ingress = *e.CounterEvent.Delta
		return
	}
	if *e.CounterEvent.Name == "dropped" && e.GetOrigin() == MetronOrigin {
		m.Metrons[metronIndex].Dropped = *e.CounterEvent.Delta
		return
	}

}

func (m *Metrics) parseEnvelope(e *events.Envelope) {

	inMem := false
	if e.GetOrigin() == MetronOrigin && e.GetEventType() == events.Envelope_CounterEvent {
		for i := range m.Metrons {
			if m.Metrons[i].Index == e.GetIndex() {
				inMem = true
				break
			}
		}

		if !inMem {
			m.Metrons = append(m.Metrons, Metron{Job: e.GetJob(), Index: e.GetIndex()})
		}
	}

	if undefinedJob(e.GetJob()) {
		return
	}
	inMem = false
	for i := range m.Instances {
		if m.Instances[i].Index == e.GetIndex() {
			inMem = true
			break
		}
	}
	if !inMem {
		m.Instances = append(m.Instances, Instance{Job: e.GetJob(), Index: e.GetIndex()})
		if e.GetJob() == DopplerJob {
			m.EnvelopeStats = append(m.EnvelopeStats, EnvelopeStat{Job: e.GetJob(), Index: e.GetIndex()})
		}
	}

	if e.GetEventType() == events.Envelope_ValueMetric {
		m.processValueMetric(e)
	}
	if e.GetEventType() == events.Envelope_CounterEvent {
		m.processCounterEvent(e)
	}
}

func undefinedJob(j string) bool {
	if j == DopplerJob || j == TrafficControllerJob || j == SyslogSchedulerJob || j == SyslogAdapterJob {
		return false
	}
	return true
}
