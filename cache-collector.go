package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"sync"
	"time"

	logcache "code.cloudfoundry.org/log-cache/pkg/client"
	"code.cloudfoundry.org/log-cache/pkg/rpc/logcache_v1"
	jwt "github.com/dgrijalva/jwt-go"
)

/*
Welcome to Firehose Analyzer - Thu Apr 25 09:42:18 CDT 2019

Job                    Instance-Counts     CPU-User     CPU-Sys     CPU-Wait      Memory
----------------------------------------------------------------------------------------
Traffic Controller     1                    2.20         1.30        0.00         35.00
Doppler                1                    3.60         2.80        0.00         68.00
Syslog Aadapter        1                    0.20         0.30        0.00         30.00
Syslog Scheduler       1                    0.30         0.30        0.00         30.00

Drain Information:
Syslog Adapter drain bindings  : 0
Syslog Scheduler drains        : 0
Doppler Sinks Dropped          : 0

Doppler Message Rate Capcity   : 898.00

Doppler						Subscriptions	Ingress	Dropped	Loss
----------------------------------------------------------------------------------------
doppler/2e771884-1b9a-43d2-a601-149935388ad5	7		898	0	0.00


Metron Health: Report any metron agents that are dropping envelopes
No unhealthy Metron Agents to report :-)



{
  "status": "success",
  "data": {
    "resultType": "vector",
    "result": [
      {
        "metric": {
          "addr": "10.1.1.31:8080",
          "deployment": "cf-0e11708a646c5524f1e2",
          "host": "",
          "id": "062d6eab-c151-4b96-8668-3e5e623010ab",
          "index": "062d6eab-c151-4b96-8668-3e5e623010ab",
          "instance-id": "",
          "ip": "10.1.1.31",
          "job": "doppler",
          "product": "Pivotal Application Service",
          "source_id": "log-cache",
          "system_domain": "system.domain"
        },
        "value": [
          1557929172,
          "218809"
        ]
      }
    ]
  }
}


CLI Range query
cf query 'ingress{source_id="doppler"}' --step 1 --start `date +'%s'` --end $(expr $(date +'%s') + 30) | jq


cf query 'rate(ingress{source_id="doppler"}[5m] offset 2m)' | jq

promql notes
https://prometheus.io/docs/prometheus/latest/querying/basics/#range-vector-selectors


./firehose-analyzer -sys run-35.haas-59.pez.pivotal.io

*/

// CacheMetric used to parse result reponse
type CacheMetric struct {
	Addr         string `json:"addr"`
	Deployment   string `json:"deployment"`
	Host         string `json:"host"`
	ID           string `json:"id"`
	Index        string `json:"index"`
	InstanceID   string `json:"instance-id"`
	IP           string `json:"ip"`
	Job          string `json:"job"`
	Product      string `json:"product"`
	SourceID     string `json:"source_id"`
	SystemDomain string `json:"system_domain"`
}

// CacheResultItem used to parse reulst reponse
type CacheResultItem struct {
	Metric CacheMetric   `json:"metric"`
	Value  []interface{} `json:"value"` // [ timestamp int, value string ]
}

// CacheData used to parse result reponse
type CacheData struct {
	ResultType string            `json:"resultType"`
	Result     []CacheResultItem `json:"result"`
}

// CacheResult response from log-cache promql query
type CacheResult struct {
	Status string    `json:"status"` // expected to be "sucess"
	Data   CacheData `json:"data"`
}

// HTTPClient client used to pass in access token for log cache requests
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type tokenHTTPClient struct {
	c           HTTPClient
	accessToken string
}

func (c *tokenHTTPClient) Do(req *http.Request) (*http.Response, error) {
	if len(c.accessToken) > 0 {
		req.Header.Set("Authorization", c.accessToken)
	}

	return c.c.Do(req)

}

// InstanceMetrics average system metrics for instance groups
type InstanceMetrics struct {
	CPUUser float64
	CPUSys  float64
	CPUWait float64
	Memory  float64
	Count   int64  // number of instances
	Name    string // name
}

type TrafficControllerMetrics struct {
	System           InstanceMetrics
	SlowConsumers    float64
	AppStreams       float64
	Egress           float64
	Ingress          float64
	ContainerLatency float64
}

type MetronMetrics struct {
	System      InstanceMetrics
	Ingress     float64
	Egress      float64
	Dropped     float64
	AVGEnvelope float64
	Name        string // name/index
}

type DopplerMetrics struct {
	System              InstanceMetrics
	MessageRateCapacity float64
	Subscriptions       float64
	Egress              float64
	Ingress             float64
	Dropped             float64
	Name                string // name/index
}

type SyslogAdapterMetrics struct {
	System InstanceMetrics
}

type SyslogSchedulerMetrics struct {
	System InstanceMetrics
}

type DrainMetrics struct {
	DrainBindings   float64
	ScheduledDrains float64
	SinksDropped    float64
}

// Metrics root of all computed metrics
type Metrics struct {
	System          []InstanceMetrics
	Doppler         DopplerMetrics
	TC              TrafficControllerMetrics
	Metron          MetronMetrics
	Drain           DrainMetrics
	DopplerInstance DopplerMetrics
	MetronInstance  MetronMetrics
	SyslogAdapter   SyslogAdapterMetrics
	SyslogScheduler SyslogSchedulerMetrics
}

// LCC used to manage log cache endoint and credentials
type LCC struct {
	accessToken string
	client      *logcache.Client
	Metric      Metrics
	mux         sync.Mutex
	Start       time.Time
	Stop        time.Time
	Offset      string
	Duration    string
}

// NewLogCacheClient createa new LCC and returns it
func NewLogCacheClient(address string) (LCC, error) {
	lc := LCC{Metric: Metrics{}}
	lc.fetchToken()
	h := http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}}
	tc := tokenHTTPClient{HTTPClient(&h), lc.accessToken}
	lc.client = logcache.NewClient(address, logcache.WithHTTPClient(&tc))
	return lc, nil
}

func (lc *LCC) fetchToken() {
	var err error
	lc.accessToken, err = cfCLI.AccessToken()
	if err != nil {
		logger.Fatalln(err)
	}
}

func (lc *LCC) checkToken() {
	t, err := jwt.Parse(lc.accessToken[7:len(lc.accessToken)], func(token *jwt.Token) (interface{}, error) { return []byte(""), nil })
	if err != nil {
		if err.Error() != jwt.ErrInvalidKeyType.Error() {
			logger.Printf("Could not parse existing token \"%s\" so  Fetching a new token\n", err)
			lc.fetchToken()
		}
	}

	claims := t.Claims.(jwt.MapClaims)
	if !claims.VerifyExpiresAt(time.Unix(time.Unix(int64(claims["exp"].(float64)), 0).Unix()-60, 0).Unix(), true) {
		lc.fetchToken() // fetch new before expire
	}
}

// GetMetric given metric and source id result is returned
func (lc *LCC) GetMetric(metric, sourceid, job string) (*logcache_v1.PromQL_InstantQueryResult, error) {
	lc.checkToken()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return lc.client.PromQL(ctx, fmt.Sprintf("%s{source_id=\"%s\",job=\"%s\"}", metric, sourceid, job))
}

// GetAvgRateMetric given range, metric and source id result is returned
func (lc *LCC) GetAvgRateMetric(metric, sourceid, job, duration, offset string) (float64, error) {
	lc.checkToken()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var result *logcache_v1.PromQL_InstantQueryResult
	var err error
	if job != "" {
		result, err = lc.client.PromQL(ctx, fmt.Sprintf("avg(rate(%s{source_id=\"%s\",job=\"%s\"}[%s] offset %s))", metric,
			sourceid,
			job,
			duration,
			offset))
	} else {
		result, err = lc.client.PromQL(ctx, fmt.Sprintf("avg(rate(%s{source_id=\"%s\"}[%s] offset %s))", metric,
			sourceid,
			duration,
			offset))
	}

	if err != nil {
		return 0.0, err
	}

	return getSingleSampleResult(result.GetVector().GetSamples()), nil
}

// GetAvgMetric promql average
func (lc *LCC) GetAvgMetric(metric, sourceid, job, offset string) (float64, error) {
	lc.checkToken()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result, err := lc.client.PromQL(ctx, fmt.Sprintf("avg(%s{source_id=\"%s\",job=\"%s\"} offset %s)", metric,
		sourceid,
		job,
		offset))

	if err != nil {
		return 0.0, err
	}
	return getSingleSampleResult(result.GetVector().GetSamples()), nil
}

// GetSumMetric promql average
func (lc *LCC) GetSumMetric(metric, sourceid, job, offset string) (float64, error) {
	lc.checkToken()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result, err := lc.client.PromQL(ctx, fmt.Sprintf("sum(%s{source_id=\"%s\",job=\"%s\"} offset %s)", metric,
		sourceid,
		job,
		offset))

	if err != nil {
		return 0.0, err
	}
	return getSingleSampleResult(result.GetVector().GetSamples()), nil
}

// Collect updates metrics from log-cache
func (lc *LCC) Collect() error {
	lc.Lock()
	defer lc.Unlock()

	lc.Offset = "2m"
	lc.Duration = "5m"

	lc.getAvgSystemMetrics(&lc.Metric.TC.System, tcJob)
	lc.getAvgSystemMetrics(&lc.Metric.Doppler.System, dopplerJob)
	lc.getAvgSystemMetrics(&lc.Metric.SyslogAdapter.System, syslogAdapterJob)
	lc.getAvgSystemMetrics(&lc.Metric.SyslogScheduler.System, syslogSchedulerJob)

	lc.sumResult(appStreamsGauge, trafficControllerSID, tcJob, &lc.Metric.TC.AppStreams)
	lc.setRateMetric(slowConsumerCounter, trafficControllerSID, tcJob, &lc.Metric.TC.SlowConsumers)

	lc.sumResult(drainBindingsGauge, syslogDrainAdapterSID, syslogAdapterJob, &lc.Metric.Drain.DrainBindings)
	lc.sumResult(drainsGauge, syslogDrainScheduleSID, syslogSchedulerJob, &lc.Metric.Drain.ScheduledDrains)
	lc.setRateMetric(droppedCounter, syslogDrainAdapterSID, syslogAdapterJob, &lc.Metric.Drain.ScheduledDrains)

	lc.setRateMetric(ingressCounter, dopplerSID, dopplerJob, &lc.Metric.Doppler.Ingress)
	lc.setRateMetric(egressCounter, dopplerSID, dopplerJob, &lc.Metric.Doppler.Egress)
	lc.setRateMetric(droppedCounter, dopplerSID, dopplerJob, &lc.Metric.Doppler.Dropped)
	lc.sumResult(subscriptionsGauge, dopplerSID, dopplerJob, &lc.Metric.Doppler.Subscriptions)

	lc.setRateMetric(ingressCounter, metronSID, "", &lc.Metric.Metron.Ingress)
	lc.setRateMetric(egressCounter, metronSID, "", &lc.Metric.Metron.Egress)
	lc.setRateMetric(droppedCounter, metronSID, "", &lc.Metric.Metron.Dropped)

	lc.Metric.Doppler.MessageRateCapacity = float64(lc.Metric.Doppler.Ingress) / float64(lc.Metric.Doppler.System.Count)

	// TODO Reverse Proxy Loss Rate
	// https://docs.pivotal.io/pivotalcf/2-5/monitoring/key-cap-scaling.html#rlp-ksi

	//logger.Printf("%v\n", *lc)
	return nil
}

// metric helpers

func (lc *LCC) setRateMetric(metric, source, job string, p *float64) {
	var err error
	*p, err = lc.GetAvgRateMetric(metric, source, job, lc.Duration, lc.Offset)
	if err != nil {
		logger.Fatalln(err)
	}
	return
}

func (lc *LCC) setInstanceCount(system *InstanceMetrics, job string) {
	result, err := lc.GetMetric(cpuUserGauge, boshSystemMetricsSID, job)
	if err != nil {
		logger.Fatalln(err)
	}
	sample := result.GetVector().GetSamples()
	system.Count = int64(len(sample))
}

func (lc *LCC) getAvgSystemMetrics(system *InstanceMetrics, job string) {
	lc.avgResult(cpuUserGauge, boshSystemMetricsSID, job, &system.CPUUser)
	lc.avgResult(cpuWaitGauge, boshSystemMetricsSID, job, &system.CPUWait)
	lc.avgResult(cpuSYSGauge, boshSystemMetricsSID, job, &system.CPUSys)
	lc.avgResult(memoryPercentGauge, boshSystemMetricsSID, job, &system.Memory)

	lc.setInstanceCount(system, job)
}

func (lc *LCC) avgResult(metric, source, job string, p *float64) {
	var err error
	*p, err = lc.GetAvgMetric(metric, source, job, lc.Offset)
	if err != nil {
		logger.Fatalln(err)
	}
	return
}

func (lc *LCC) sumResult(metric, source, job string, p *float64) {
	var err error
	*p, err = lc.GetSumMetric(metric, source, job, lc.Offset)
	if err != nil {
		logger.Fatalln(err)
	}
}

func (lc *LCC) Lock() {
	lc.mux.Lock()
}

func (lc *LCC) Unlock() {
	lc.mux.Unlock()
}

func getSingleSampleResult(sample []*logcache_v1.PromQL_Sample) float64 {
	for i := range sample {
		return sample[i].GetPoint().GetValue()
	}
	return 0.0
}
