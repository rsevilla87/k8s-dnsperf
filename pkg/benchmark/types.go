package benchmark

import (
	"time"

	"k8s-dnsperf.io/pkg/infra"
)

// Represents a benchmark instance
type Benchmark struct {
	duration int
	clients  int
	timeout  int
	server   string
	port     int
	infra    *infra.Infra
}

// Represents dnsperf results with required metadata
type Result struct {
	UUID          string        `json:"uuid"`
	Clients       int           `json:"clients"`
	Records       int           `json:"records"`
	Timestamp     time.Time     `json:"timestamp"`
	Duration      time.Duration `json:"duration"`
	TargetServer  string        `json:"target_server"`
	QSent         float64       `json:"queries_sent"`
	QCompleted    float64       `json:"queries_completed"`
	QLost         float64       `json:"queries_lost"`
	QInterrupted  float64       `json:"queries_interrupted"`
	ResponseCodes float64       `json:"response_codes"`
	QPS           float64       `json:"qps"`
	AvgLatency    float64       `json:"avg_latency_ms"`
	MaxLatency    float64       `json:"max_latency_ms"`
	MinLatency    float64       `json:"min_latency_ms"`
	LatencyStdev  float64       `json:"latency_stdev_ms"`
}

type podResult struct {
	QSent         float64
	QCompleted    float64
	QLost         float64
	QInterrupted  float64
	ResponseCodes float64
	QPS           float64
	AvgLatency    float64
	MaxLatency    float64
	MinLatency    float64
	LatencyStdev  float64
}
