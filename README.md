# k8s-dnsperf

Kubernetes DNS loader

## Running

k8s-dnsperf is a tool that helps to run [dnsperf](https://github.com/DNS-OARC/dnsperf) in a kubernetes cluster. It provides multiple features.

A regular k8s-dnsperf execution consists of several stages:

- Deploy benchmark assets
  - k8s-dnsperf namespace
  - k8s-dnsperf DaemonSet, initially in idle status
  - ClusterIP Services, each DNS record is generated from a k8s service, first record will always be `kubernetes.default.svc.cluster.local`.
  - ConfigMap holding the previous records
- Run dnsperf benchmark using the configured flags in parallel against the target DNS server, by default `172.30.0.10`
- Parse dnsperf output
- Index results. (Indexing is optional)
- Destroy assets

```shell
$ ./bin/k8s-dnsperf -duration 1m --es-server ${ES_SERVER}
2024-10-08 13:00:27 INF cmd/k8s-dnsperf.go:41 > Starting k8s-dnsperf main@e056503b9e2c6a4db7e7b02e4cf2d83220b668c9
2024-10-08 13:00:27 INF cmd/k8s-dnsperf.go:43 > Creating indexer 💾
2024-10-08 13:00:29 INF pkg/infra/infrastructure.go:56 > Creating benchmark assets 🚧
2024-10-08 13:00:30 INF pkg/infra/infrastructure.go:106 > Waiting for DaemonSet k8s-dnsperf/k8s-dnsperf pods to be running
2024-10-08 13:00:36 INF pkg/benchmark/dnsperf.go:40 > Running benchmark 🚀
...
```

## Prometheus metrics

Several prometheus metrics and expressions can be used to monitor CoreDNS performance, some of the most interesting ones are described below:

- `sum(irate(container_memory_rss{name=~"k8s_dns_dns.+",namespace=~"openshift-dns"}[2m])) * 100`: Cluster CoreDNS aggregated CPU usage (100% means 1 core)
- `irate(container_memory_rss{name=~"k8s_dns_dns.+",namespace=~"openshift-dns"}[2m])`: Per instance CoreDNS CPU usage
- `sum(irate(coredns_dns_request_count_total{}[2m])) by (job)`: Aggregated CoreDNS queries per second
- `irate(coredns_dns_request_count_total{}[2m])`: Per instance CoreDNS queries per second
- `container_memory_rss{name=~"k8s_dns_dns.+",namespace=~"openshift-dns"}`: Per instance CoreDNS RSS memory in bytes
- `sum(container_memory_rss{name=~"k8s_dns_dns.+",namespace=~"openshift-dns"})`: Aggregated CoreDNS RSS memory in bytes
- `histogram_quantile(0.99, rate(coredns_dns_request_duration_seconds_bucket[2m]))`: P99 request latency in seconds
- `rate(coredns_cache_hits_total[2m])`: Cache hits rate
- `rate(coredns_cache_misses_total[2m])`: Cache miss rate

## Building

Requires go 1.22 or greater and make.
