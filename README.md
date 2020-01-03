## Firehose Aanalyzer

Collects a summary of firehose activity by reading ValueMetric and CountEvent metrics from the firehose.

### Instance stats

Averages the cpu and memory stats accorss instance groups.

### Drain Information

Reports how many syslog drains are configured and how many are actually bound.

### Doppler 

Reports subscription, ingress, and dropped metrics for each doppler instance.

### Metron and Reverse Log Proxy Health
Reports ingress, and dropped metrics for metron and reverse log proxy jobs.

### install

```
cf install-plugin https://github.com/randomtask1155/firehose-analyzer/releases/download/v1.0.0/firehose-analyzer.osx-1.0.0
```

### Usage

```
cf firehose-analyzer
```

### Demo

[![asciicast](https://asciinema.org/a/pxJsQJm1SWTT0hmR8vhJEyjez.svg)](https://asciinema.org/a/pxJsQJm1SWTT0hmR8vhJEyjez)


### Queries

Here is a list of queries the firehose-analyzer will execute.  You can run these sample queries using `cf query` command.

#### System cpu metrics.  

There are three metrics system_cpu_user, system_cpu_wait, and system_cpu_sys

`'avg(rate(system_cpu_user{source_id="bosh-system-metrics-forwarder",job="loggregator_trafficcontroller"}[5m]2m))'`

#### Syslog Drain Metrics

Number of drain Bindings

`'sum(drain_bindings{source_id="drain_adapter",job="syslog_adapter"} offset 2m)'`

Number of syslog drain drops

`'sum(dropped{source_id="drain_adapter",job="syslog_adapter"} offset 2m)'`

Number of scheduled drains

`'sum(drains{source_id="drain_scheduler",job="syslog_scheduler"} offset 2m)'`

#### TrafficController Metrics

Number of App Streams

`'sum(doppler_proxy_app_streams{source_id="traffic_controller",job="loggregator_trafficcontroller"} offset 2m)'`

Average Slow Consumer Rate

`'avg(rate(doppler_proxy_slow_consumer{source_id="traffic_controller",job="loggregator_trafficcontroller"}[5m] offset 2m))'`

#### Doppler Metrics

Sum Ingress Rate

`'sum(rate(ingress{source_id="doppler",job="doppler"}[5m] offset 2m))'`

Maximum Ingress Dropped for given duration

`'sum(max_over_time(dropped{source_id="doppler", direction="ingress"}[5m])) by (index) > 0'`

Sum Egress Rate

`'sum(rate(egress{source_id="doppler",job="doppler"}[5m] offset 2m))'`

Sum of Dropped rate 

`'sum(rate(dropped{source_id="doppler",job="doppler"}[5m] offset 2m))'`

Number of Doppler Subscriptions

`'sum(subscriptions{source_id="doppler",job="doppler"} offset 2m)'`

#### Metron Metrics

Sum Ingress rate across all metron/loggregator agents

`'sum(rate(ingress{source_id="metron"}[5m] offset 2m))'`

Sum Egress rate across all metron agents

`'sum(rate(ingress{source_id="metron"}[5m] offset 2m))'`

Sum Rate of dropped envelopes

`'sum(rate(dropped{source_id="metron"}[5m] offset 2m))'`

#### Reverse Log Proxy Metrics

Sum ingress Rate

`'sum(rate(ingress{source_id="reverse_log_proxy",job="loggregator_trafficcontroller"}[5m] offset 2m))'`

Sum egress Rate

`'sum(rate(egress{source_id="reverse_log_proxy",job="loggregator_trafficcontroller"}[5m] offset 2m))'`

Sum of rate of drops

`'sum(rate(dropped{source_id="reverse_log_proxy"}[5m] offset 2m))'`