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