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
SOME NOTES
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

// TrafficControllerMetrics TC metrics
type TrafficControllerMetrics struct {
	System           InstanceMetrics
	SlowConsumers    float64
	AppStreams       float64
	Egress           float64
	Ingress          float64
	ContainerLatency float64
}

type RLPMetrics struct {
	Egress  float64
	Ingress float64
	Dropped float64
}

// MetronMetrics metron metrics
type MetronMetrics struct {
	System      InstanceMetrics
	Ingress     float64
	Egress      float64
	Dropped     float64
	AVGEnvelope float64
	Name        string // name/index
}

// DopplerMetrics doppler metrics
type DopplerMetrics struct {
	System              InstanceMetrics
	MessageRateCapacity float64
	Subscriptions       float64
	Egress              float64
	Ingress             float64
	IngressDropped      float64
	Dropped             float64
	Name                string // name/index
}

// SyslogAdapterMetrics syslog adapter metrics
type SyslogAdapterMetrics struct {
	System InstanceMetrics
}

// SyslogSchedulerMetrics syslog scheduler metrics
type SyslogSchedulerMetrics struct {
	System InstanceMetrics
}

// DrainMetrics Drain metrics
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
	RLP             RLPMetrics
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

var (
	queryAvgRateJob         string
	queryAvgRate            string
	querySumRateJob         string
	querySumRate            string
	querySumJob             string
	queryMetricJob          string
	queryIngressMaxOverTime string
)

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

// GetResult given metric and source id result is returned
func (lc *LCC) GetResult(metric, sourceid, job, q string) (*logcache_v1.PromQL_InstantQueryResult, error) {
	lc.checkToken()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var result *logcache_v1.PromQL_InstantQueryResult
	var err error
	if job != "" {
		result, err = lc.client.PromQL(ctx, fmt.Sprintf(q, metric, sourceid, job))
	} else {
		result, err = lc.client.PromQL(ctx, fmt.Sprintf(q, metric, sourceid))
	}

	return result, err
}

// GetSinlgeMetric given range, metric and source id result is returned
func (lc *LCC) GetSinlgeMetric(metric, sourceid, job, q string) float64 {
	lc.checkToken()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var result *logcache_v1.PromQL_InstantQueryResult
	var err error
	if job != "" {
		result, err = lc.client.PromQL(ctx, fmt.Sprintf(q, metric, sourceid, job))
	} else {
		result, err = lc.client.PromQL(ctx, fmt.Sprintf(q, metric, sourceid))
	}

	if err != nil {
		if job != "" {
			logger.Printf(q, metric, sourceid, job)
		} else {
			logger.Printf(q, metric, sourceid)
		}
		logger.Fatalln(err)
	}

	return getSingleSampleResult(result.GetVector().GetSamples())
}

// Collect updates metrics from log-cache
func (lc *LCC) Collect() error {
	lc.Lock()
	defer lc.Unlock()

	lc.updateQeries("2m", "5m")

	lc.getAvgSystemMetrics(&lc.Metric.TC.System, tcJob)
	lc.getAvgSystemMetrics(&lc.Metric.Doppler.System, dopplerJob)
	lc.getAvgSystemMetrics(&lc.Metric.SyslogAdapter.System, syslogAdapterJob)
	lc.getAvgSystemMetrics(&lc.Metric.SyslogScheduler.System, syslogSchedulerJob)

	lc.Metric.TC.AppStreams = lc.GetSinlgeMetric(appStreamsGauge, trafficControllerSID, tcJob, querySumJob)
	lc.Metric.TC.SlowConsumers = lc.GetSinlgeMetric(slowConsumerCounter, trafficControllerSID, tcJob, queryAvgRateJob)

	lc.Metric.Drain.DrainBindings = lc.GetSinlgeMetric(drainBindingsGauge, syslogDrainAdapterSID, syslogAdapterJob, querySumJob)
	lc.Metric.Drain.ScheduledDrains = lc.GetSinlgeMetric(drainsGauge, syslogDrainScheduleSID, syslogSchedulerJob, querySumJob)
	lc.Metric.Drain.ScheduledDrains = lc.GetSinlgeMetric(droppedCounter, syslogDrainAdapterSID, syslogAdapterJob, querySumRateJob)

	lc.Metric.Doppler.Ingress = lc.GetSinlgeMetric(ingressCounter, dopplerSID, dopplerJob, querySumRateJob)
  lc.Metric.Doppler.IngressDropped = lc.GetSinlgeMetric(droppedCounter, dopplerSID, "", queryIngressMaxOverTime)
	lc.Metric.Doppler.Egress = lc.GetSinlgeMetric(egressCounter, dopplerSID, dopplerJob, querySumRateJob)
	lc.Metric.Doppler.Dropped = lc.GetSinlgeMetric(droppedCounter, dopplerSID, dopplerJob, querySumRateJob)
	lc.Metric.Doppler.Subscriptions = lc.GetSinlgeMetric(subscriptionsGauge, dopplerSID, dopplerJob, querySumJob)

	lc.Metric.Metron.Ingress = lc.GetSinlgeMetric(ingressCounter, metronSID, "", querySumRateJob)
	lc.Metric.Metron.Egress = lc.GetSinlgeMetric(egressCounter, metronSID, "", querySumRateJob)
	lc.Metric.Metron.Dropped = lc.GetSinlgeMetric(droppedCounter, metronSID, "", querySumRateJob)

	lc.Metric.RLP.Ingress = lc.GetSinlgeMetric(ingressCounter, rlpSID, tcJob, querySumRateJob)
	lc.Metric.RLP.Egress = lc.GetSinlgeMetric(egressCounter, rlpSID, tcJob, querySumRateJob)
	lc.Metric.RLP.Dropped = lc.GetSinlgeMetric(droppedCounter, rlpSID, tcJob, querySumRateJob)

	lc.Metric.Doppler.MessageRateCapacity = float64(lc.Metric.Doppler.Ingress) / float64(lc.Metric.Doppler.System.Count)
	return nil
}

func (lc *LCC) updateQeries(offset, duration string) {
	lc.Offset = offset
	lc.Duration = duration
	queryAvgRateJob = "avg(rate(%s{source_id=\"%s\",job=\"%s\"}[" + lc.Duration + "] offset " + lc.Offset + "))"
	queryAvgRate = "avg(rate(%s{source_id=\"%s\"}[" + lc.Duration + "] offset " + lc.Offset + "))"
	querySumRateJob = "sum(rate(%s{source_id=\"%s\",job=\"%s\"}[" + lc.Duration + "] offset " + lc.Offset + "))"
	querySumRate = "sum(rate(%s{source_id=\"%s\"}[" + lc.Duration + "] offset " + lc.Offset + "))"
	querySumJob = "sum(%s{source_id=\"%s\",job=\"%s\"} offset " + lc.Offset + ")"
	queryMetricJob = "%s{source_id=\"%s\",job=\"%s\"}"
	queryIngressMaxOverTime = "sum(max_over_time(%s{source_id=\"%s\", direction=\"ingress\"}[" + lc.Duration + "])) by (index) > 0"
}

// metric helpers
func (lc *LCC) setInstanceCount(system *InstanceMetrics, job string) {
	result, err := lc.GetResult(cpuUserGauge, boshSystemMetricsSID, job, queryMetricJob)
	if err != nil {
		logger.Fatalln(err)
	}
	sample := result.GetVector().GetSamples()
	system.Count = int64(len(sample))
}

func (lc *LCC) getAvgSystemMetrics(system *InstanceMetrics, job string) {
	system.CPUUser = lc.GetSinlgeMetric(cpuUserGauge, boshSystemMetricsSID, job, queryAvgRateJob)
	system.CPUWait = lc.GetSinlgeMetric(cpuWaitGauge, boshSystemMetricsSID, job, queryAvgRateJob)
	system.CPUSys = lc.GetSinlgeMetric(cpuSYSGauge, boshSystemMetricsSID, job, queryAvgRateJob)
	system.Memory = lc.GetSinlgeMetric(memoryPercentGauge, boshSystemMetricsSID, job, queryAvgRateJob)

	lc.setInstanceCount(system, job)
}

// Lock used to lock when updating/reading metrics
func (lc *LCC) Lock() {
	lc.mux.Lock()
}

// Unlock used to unlock after reading/updating metrics
func (lc *LCC) Unlock() {
	lc.mux.Unlock()
}

func getSingleSampleResult(sample []*logcache_v1.PromQL_Sample) float64 {
	for i := range sample {
		return sample[i].GetPoint().GetValue()
	}
	return 0.0
}
