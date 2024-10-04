package benchmark

import (
	"time"

	"k8s-dnsperf.io/pkg/infra"
)

type Benchmark struct {
	duration int
	clients  int
	timeout  int
	server   string
	infra    infra.Infra
}

// Represents dnsperf results with required metadata
type Result struct {
	UUID          string        `json:"uuid"`
	Clients       int           `json:"clients"`
	Timestamp     string        `json:"timestamp"`
	Duration      time.Duration `json:"duration"`
	TestType      string        `json:"test_type"`
	QSent         float64       `json:"queries_sent"`
	QCompleted    float64       `json:"queries_completed"`
	QLost         float64       `json:"queries_lost"`
	QInterrupted  float64       `json:"queries_interrupted"`
	ResponseCodes float64       `json:"response_codes"`
	QPS           float64       `json:"qps"`
	AvgLatency    float64       `json:"avg_latency_s"`
	MaxLatency    float64       `json:"max_latency_s"`
	MinLatency    float64       `json:"min_latency_s"`
	LatencyStdev  float64       `json:"latency_stdev"`
}
