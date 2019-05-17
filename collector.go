package main

/*
import (
	"github.com/cloudfoundry/sonde-go/events"
)
/
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
	SinksDropped  uint64
	Ingress       uint64
	Dropped       uint64
}

// Metron stats related to metron sent/received envelopes.   look for agents that are receiving more than 8k (80% of the max of 10k)
type Metron struct {
	Job     string
	Index   string
	Ingress uint64
	Dropped uint64
}

type InstanceMetrics struct {
	CPUUser
	CPUSys

	Memory
	Name
}
type Metrics struct {

}

func (m *Metrics) processValueMetric(e *events.Envelope) bool {
	index := e.GetIndex()
	instanceIndex := -1
	for i := range m.Instances {
		if m.Instances[i].Index == index {
			instanceIndex = i
			break
		}
	}

	if *e.ValueMetric.Name == "system.cpu.wait" && e.GetOrigin() == "bosh-system-metrics-forwarder" {
		m.Instances[instanceIndex].CPUWait = e.ValueMetric.GetValue()
		return true
	}
	if *e.ValueMetric.Name == "system.cpu.user" && e.GetOrigin() == "bosh-system-metrics-forwarder" {
		m.Instances[instanceIndex].CPUUser = e.ValueMetric.GetValue()
		return true
	}
	if *e.ValueMetric.Name == "system.cpu.sys" && e.GetOrigin() == "bosh-system-metrics-forwarder" {
		m.Instances[instanceIndex].CPUSys = e.ValueMetric.GetValue()
		return true
	}
	if *e.ValueMetric.Name == "system.mem.percent" && e.GetOrigin() == "bosh-system-metrics-forwarder" {
		m.Instances[instanceIndex].Memory = e.ValueMetric.GetValue()
		return true
	}

	if *e.ValueMetric.Name == "drains" && e.GetOrigin() == "cf-syslog-drain.scheduler" {
		m.SchedulerDrains = e.ValueMetric.GetValue()
		return true
	}

	if *e.ValueMetric.Name == "drain_bindings" && e.GetOrigin() == "cf-syslog-drain.adapter" {
		m.AdapterDrainBindings = e.ValueMetric.GetValue()
		return true
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
		m.EnvelopeStats[dopplerIndex].Subscriptions = e.ValueMetric.GetValue()
		return true
	}
	return false
}

func (m *Metrics) processCounterEvent(e *events.Envelope) bool {
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
			break
		}
	}

	if *e.CounterEvent.Name == "ingress" && e.GetOrigin() == "loggregator.doppler" {
		m.EnvelopeStats[dopplerIndex].Ingress = e.CounterEvent.GetDelta()
		return true
	}
	if *e.CounterEvent.Name == "dropped" && e.GetOrigin() == "loggregator.doppler" {
		m.EnvelopeStats[dopplerIndex].Dropped = e.CounterEvent.GetDelta()
		return true
	}
	if *e.CounterEvent.Name == "sinks.dropped" && e.GetOrigin() == "loggregator.doppler" {
		m.EnvelopeStats[dopplerIndex].SinksDropped = e.CounterEvent.GetDelta()
		return true
	}

	if *e.CounterEvent.Name == "ingress" && e.GetOrigin() == MetronOrigin {
		m.Metrons[metronIndex].Ingress = e.CounterEvent.GetDelta()
		return true
	}
	if *e.CounterEvent.Name == "dropped" && e.GetOrigin() == MetronOrigin {
		m.Metrons[metronIndex].Dropped = e.CounterEvent.GetDelta()
		return true
	}
	return false
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
		if m.processValueMetric(e) && arvhiveEnabled {
			go archiveMetric(e, e.ValueMetric.GetName(), e.ValueMetric.GetValue(), e.ValueMetric.GetUnit())
		}
	}
	if e.GetEventType() == events.Envelope_CounterEvent {
		if m.processCounterEvent(e) && arvhiveEnabled {
			go archiveMetric(e, e.CounterEvent.GetName(), e.CounterEvent.GetDelta(), e.CounterEvent.GetTotal())
		}
	}
}

func undefinedJob(j string) bool {
	if j == DopplerJob || j == TrafficControllerJob || j == SyslogSchedulerJob || j == SyslogAdapterJob {
		return false
	}
	return true
}
*/
