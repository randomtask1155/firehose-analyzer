## Firehose Aanalyzer

Collects a summary of firehose activity by reading ValueMetric and CountEvent metrics from the firehose.

### Instance stats

Averages the cpu and memory stats accorss instance groups.

### Drain Information

Reports how many syslog drains are configured and how many are actually bound.

### Doppler 

Reports subscription, ingress, and dropped metrics for each doppler instance.

### Metron Health
If a metron agent reports any dropped envelopes this section will display the job and index information.

### Usage

```
cf firehose-analyzer
```

### Demo

[![asciicast](https://asciinema.org/a/214731.svg)](https://asciinema.org/a/214731)