package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"os/exec"
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

func checkAPI() error {
	cfcli, err := exec.LookPath("cf")
	if err != nil {
		return fmt.Errorf("cf cli lookup failed: %s", err)
	}
	// run cf spaces to force a refresh of access token
	authOut, err := exec.Command(cfcli, "spaces").CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %s", authOut, err)
	}
	_, err = exec.Command(cfcli, "curl", "/v2/info").Output()
	if err != nil {
		return err
	}
	return err
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
	SlowConsumers    int64
	AppStreams       int64
	Egress           int64
	Ingress          int64
	ContainerLatency float64
}

type MetronMetrics struct {
	System      InstanceMetrics
	Ingress     int64
	Egress      int64
	Dropped     int64
	AVGEnvelope float64
	Name        string // name/index
}

type DopplerMetrics struct {
	System              InstanceMetrics
	MessageRateCapacity float64
	Subscriptions       int64
	Egress              int64
	Ingress             int64
	Dropped             int64
	Name                string // name/index
}

type SyslogAdapterMetrics struct {
	System InstanceMetrics
}

type SyslogSchedulerMetrics struct {
	System InstanceMetrics
}

type DrainMetrics struct {
	DrainBindings   int64
	ScheduledDrains int64
	SinksDropped    int64
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
}

// NewLogCacheClient createa new LCC and returns it
func NewLogCacheClient(address string) (LCC, error) {
	lc := LCC{Metric: Metrics{}}
	err := lc.fetchToken()
	if err != nil {
		return lc, err
	}
	h := http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}}
	tc := tokenHTTPClient{HTTPClient(&h), lc.accessToken}
	lc.client = logcache.NewClient(address, logcache.WithHTTPClient(&tc))
	return lc, nil
}

func (lc *LCC) checkToken() bool {
	if lc.accessToken == "" {
		return false
	}
	t, err := jwt.Parse(lc.accessToken[7:len(lc.accessToken)], func(token *jwt.Token) (interface{}, error) { return []byte(""), nil })
	if err != nil {
		if err.Error() != jwt.ErrInvalidKeyType.Error() {
			logger.Printf("Could not parse existing token \"%s\" so  Fetching a new token\n", err)
			*accessToken = ""
			lc.accessToken = ""
			err := lc.fetchToken()
			if err != nil {
				logger.Printf("Could not fetch new token: %s\n", err)
				return false
			} else {
				return true
			}
		}
	}

	claims := t.Claims.(jwt.MapClaims)
	if !claims.VerifyExpiresAt(time.Unix(time.Unix(int64(claims["exp"].(float64)), 0).Unix()-60, 0).Unix(), true) {
		// access token is going to expire or is expired so fetch a new one
		lc.accessToken = ""
		*accessToken = ""
		err = lc.fetchToken()
		if err != nil {
			fmt.Printf("Could not fetch new token: %s\n", err)
			return false
		}
	}
	return true
}

func (lc *LCC) fetchToken() error {

	if *accessToken != "" {
		lc.accessToken = *accessToken
		return nil
	}

	//TODO check api target and logcache system domains match

	cfcli, err := exec.LookPath("cf")
	if err != nil {
		return fmt.Errorf("cf cli lookup failed: %s", err)
	}

	authOut, err := exec.Command(cfcli, "spaces").CombinedOutput()
	if err != nil {
		return fmt.Errorf("Make sure you have recently logged in with cf cli: %s: %s", authOut, err)
	}
	_, err = exec.Command(cfcli, "curl", "/v2/info").Output()
	if err != nil {
		return fmt.Errorf("Make sure you have recently logged in with cf cli: %s", err)
	}

	token, err := exec.Command(cfcli, "oauth-token").CombinedOutput()
	if err != nil {
		return fmt.Errorf("Make sure you have recently logged in with cf cli: %s", err)
	}

	lc.accessToken = fmt.Sprintf("%s", token[0:len(token)-1])
	return nil
}

// GetMetric given metric and source id result is returned
func (lc *LCC) GetMetric(metric, sourceid string) (*logcache_v1.PromQL_InstantQueryResult, error) {
	if !lc.checkToken() {
		return new(logcache_v1.PromQL_InstantQueryResult), fmt.Errorf("access token invalid")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return lc.client.PromQL(ctx, fmt.Sprintf("%s{source_id=\"%s\"}", metric, sourceid))
}

// GetRateMetric given range, metric and source id result is returned
func (lc *LCC) GetRateMetric(start, stop, metric, sourceid, job, offset string, duration int) (*logcache_v1.PromQL_InstantQueryResult, error) {
	if !lc.checkToken() {
		return new(logcache_v1.PromQL_InstantQueryResult), fmt.Errorf("access token invalid")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return lc.client.PromQL(ctx, fmt.Sprintf("%s{source_id=\"%s\",job=\"%s\"}[%d] offset %s", metric,
		sourceid,
		job,
		duration,
		offset))
}

// GetAVGMetric promql average
func (lc *LCC) GetAVGMetric(start, stop, metric, sourceid, job, offset string) (*logcache_v1.PromQL_InstantQueryResult, error) {
	if !lc.checkToken() {
		return new(logcache_v1.PromQL_InstantQueryResult), fmt.Errorf("access token invalid")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return lc.client.PromQL(ctx, fmt.Sprintf("%s{source_id=\"%s\",job=\"%s\"} offset %s", metric,
		sourceid,
		job,
		offset))
}

// Collect updates metrics from log-cache
func (lc *LCC) Collect() error {
	lc.Lock()
	defer lc.Unlock()
	lc.getAvgSystemMetrics(&lc.Metric.TC.System, tcJob)
	lc.getAvgSystemMetrics(&lc.Metric.Doppler.System, dopplerJob)
	lc.getAvgSystemMetrics(&lc.Metric.SyslogAdapter.System, syslogAdapterJob)
	lc.getAvgSystemMetrics(&lc.Metric.SyslogScheduler.System, syslogSchedulerJob)

	lc.sumResult(appStreamsGauge, trafficControllerSID, tcJob, &lc.Metric.TC.AppStreams)
	lc.sumResult(slowConsumerCounter, trafficControllerSID, tcJob, &lc.Metric.TC.SlowConsumers)

	lc.sumResult(drainBindingsGauge, syslogDrainAdapterSID, syslogAdapterJob, &lc.Metric.Drain.DrainBindings)
	lc.sumResult(drainsGauge, syslogDrainScheduleSID, syslogSchedulerJob, &lc.Metric.Drain.ScheduledDrains)
	lc.sumResult(droppedCounter, syslogDrainAdapterSID, syslogAdapterJob, &lc.Metric.Drain.ScheduledDrains)

	lc.sumResult(ingressCounter, dopplerSID, dopplerJob, &lc.Metric.Doppler.Ingress)
	lc.sumResult(egressCounter, dopplerSID, dopplerJob, &lc.Metric.Doppler.Egress)
	lc.sumResult(droppedCounter, dopplerSID, dopplerJob, &lc.Metric.Doppler.Dropped)
	lc.sumResult(subscriptionsGauge, dopplerSID, dopplerJob, &lc.Metric.Doppler.Subscriptions)

	lc.Metric.Metron.System.Count = lc.sumResult(ingressCounter, metronSID, metronJob, &lc.Metric.Metron.Ingress)
	lc.sumResult(egressCounter, metronSID, metronJob, &lc.Metric.Metron.Egress)
	lc.sumResult(droppedCounter, metronSID, metronJob, &lc.Metric.Metron.Dropped)

	lc.Metric.Doppler.MessageRateCapacity = float64(lc.Metric.Doppler.Ingress) / float64(lc.Metric.Doppler.System.Count)

	// TODO Reverse Proxy Loss Rate
	// https://docs.pivotal.io/pivotalcf/2-5/monitoring/key-cap-scaling.html#rlp-ksi

	//logger.Printf("%v\n", *lc)
	return nil
}

// metric helpers

func (lc *LCC) getAvgSystemMetrics(system *InstanceMetrics, job string) {
	system.Count = lc.avgResult(cpuUserGauge, boshSystemMetricsSID, job, &system.CPUUser)
	lc.avgResult(cpuWaitGauge, boshSystemMetricsSID, job, &system.CPUWait)
	lc.avgResult(cpuSYSGauge, boshSystemMetricsSID, job, &system.CPUSys)
	lc.avgResult(memoryPercentGauge, boshSystemMetricsSID, job, &system.Memory)
}

func (lc *LCC) avgResult(metric, source, job string, p *float64) int64 {
	result, err := lc.GetMetric(metric, source)
	if err != nil {
		logger.Fatalln(err)
	}
	sample := result.GetVector().GetSamples()
	var sum float64
	var count int64
	for i := range sample {
		if sample[i].GetMetric()["job"] == job {
			sum += sample[i].GetPoint().GetValue()
			count++
		}
	}
	*p = sum / float64(count)
	return count
}

func (lc *LCC) sumResult(metric, source, job string, p *int64) int64 {
	result, err := lc.GetMetric(metric, source)
	if err != nil {
		logger.Fatalln(err)
	}
	sample := result.GetVector().GetSamples()
	var sum float64
	var count int64
	for i := range sample {
		if sample[i].GetMetric()["job"] == job {
			sum += sample[i].GetPoint().GetValue()
			count++
		}
	}
	*p = int64(sum)
	return count
}

func (lc *LCC) Lock() {
	lc.mux.Lock()
}

func (lc *LCC) Unlock() {
	lc.mux.Unlock()
}
