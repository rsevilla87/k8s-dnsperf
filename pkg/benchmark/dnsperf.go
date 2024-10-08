package benchmark

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"

	"regexp"
	"strconv"
	"time"

	"k8s-dnsperf.io/pkg/infra"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/kubectl/pkg/scheme"
)

func NewBenchmark(infra *infra.Infra, clients int, server string, port int, timeout, duration time.Duration) Benchmark {
	// TODO validate fields
	return Benchmark{
		duration: int(duration.Seconds()),
		clients:  clients,
		timeout:  int(timeout.Seconds()),
		server:   server,
		port:     port,
		infra:    infra,
	}
}

// Runs dnsperf benchmark against a list of pods
func (b *Benchmark) Run() (Result, error) {
	var benchmarkResult Result
	var podResults []podResult
	errGroup := errgroup.Group{}
	log.Info().Msgf("Running benchmark 🚀")
	for _, pod := range b.infra.ClientPods.Items {
		errGroup.Go(func() error {
			res, err := b.exec(context.TODO(), pod)
			if err != nil {
				return err
			}
			podResults = append(podResults, res)
			return nil
		})
	}
	if err := errGroup.Wait(); err != nil {
		return benchmarkResult, err
	}
	return b.aggregateStats(podResults), nil
}

func (b *Benchmark) exec(ctx context.Context, pod corev1.Pod) (podResult, error) {
	var result podResult
	var stdout, stderr bytes.Buffer
	command := fmt.Sprintf("dnsperf -s %s -p %d -l %d -c %d -t %d -d /records", b.server, b.port, b.duration, b.timeout, b.clients)
	log.Debug().Msgf("Running command in %s: %s", pod.Name, command)
	req := b.infra.ClientSet.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(pod.Name).
		Namespace(pod.Namespace).
		SubResource("exec")
	req.VersionedParams(&corev1.PodExecOptions{
		Container: infra.K8sDNSPerf,
		Stdin:     false,
		Stdout:    true,
		Stderr:    true,
		Command:   strings.Fields(command),
		TTY:       false,
	}, scheme.ParameterCodec)
	exec, err := remotecommand.NewSPDYExecutor(b.infra.RestConfig, "POST", req.URL())
	if err != nil {
		return result, err
	}
	err = exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdout: &stdout,
		Stderr: &stderr,
	})
	if err != nil {
		log.Error().Msgf("Exec failed in pod %s: %v, stdout: %v", pod.Name, err.Error(), stdout.String())
		return result, err
	}
	log.Trace().Msgf("Output from %s: %s", pod.Name, stdout.String())
	return b.parse(stdout.String()), nil
}

/*
Parses dnsperf output and returns Result

Example output:
$ dnsperf -l 5 -c 10 -d input -c 1 -s 127.0.0.53
DNS Performance Testing Tool
Version 2.12.0

[Status] Command line: dnsperf -l 5 -c 10 -d input -c 1 -s 127.0.0.53
[Status] Sending queries (to 127.0.0.53:53)
[Status] Started at: Mon Sep 30 12:15:58 2024
[Status] Stopping after 5.000000 seconds
[Status] Testing complete (time limit)

Statistics:

	Queries sent:         52083
	Queries completed:    52083 (100.00%)
	Queries lost:         0 (0.00%)

	Response codes:       NOERROR 26042 (50.00%), NXDOMAIN 26041 (50.00%)
	Average packet size:  request 37, response 82
	Run time (s):         5.009131
	Queries per second:   10397.611881

	Average Latency (s):  0.009546 (min 0.004271, max 0.167582)
	Latency StdDev (s):   0.005814
*/
func (b *Benchmark) parse(stdout string) podResult {
	var result podResult
	// Regular expressions for extracting the numbers and other details
	reSent := regexp.MustCompile(`Queries sent:\s+(\d+)`)
	reCompleted := regexp.MustCompile(`Queries completed:\s+(\d+)`)
	reLost := regexp.MustCompile(`Queries lost:\s+(\d+)`)
	reInterrupted := regexp.MustCompile(`Queries interrupted:\s+(\d+)`)
	//reResponseCodes := regexp.MustCompile(`Response codes:\s+(.+)`)
	reQPS := regexp.MustCompile(`Queries per second:\s+([\d.]+)`)
	reAvgLatency := regexp.MustCompile(`Average Latency\s.+:\s+([\d.]+)`)
	reMinLatency := regexp.MustCompile(`Average Latency.+min ([\d.]+)`)
	reMaxLatency := regexp.MustCompile(`Average Latency.+max ([\d.]+)`)
	reLatencyStdDev := regexp.MustCompile(`Latency StdDev \(s\):\s+([\d.]+)`)
	evalRegex := func(regex *regexp.Regexp, notFoundIsError bool) float64 {
		var v float64
		var err error
		res := regex.FindStringSubmatch(stdout)
		if notFoundIsError && len(res) != 2 {
			log.Fatal().Msgf("Regex error: %s", regex.String())
			return v
		}
		if len(res) > 0 {
			v, err = strconv.ParseFloat(res[1], 64)
			if err != nil {
				log.Fatal().Msgf("String conversion error: %s", err)
				return v
			}
		}
		return v
	}
	// Extract the numbers and details
	result.QSent = evalRegex(reSent, true)
	result.QCompleted = evalRegex(reCompleted, true)
	result.QLost = evalRegex(reLost, false)
	result.QInterrupted = evalRegex(reInterrupted, false)
	//result.ResponseCodes = evalRegex(reResponseCodes)
	result.QPS = evalRegex(reQPS, true)
	result.AvgLatency = evalRegex(reAvgLatency, true) * 1000
	result.MinLatency = evalRegex(reMinLatency, true) * 1000
	result.MaxLatency = evalRegex(reMaxLatency, true) * 1000
	result.LatencyStdev = evalRegex(reLatencyStdDev, true) * 1000
	// TODO latency-histogram
	return result
}

func (b *Benchmark) aggregateStats(results []podResult) Result {
	result := Result{
		UUID:         b.infra.UUID,
		Clients:      b.clients,
		Timestamp:    time.Now(),
		Duration:     time.Duration(b.duration),
		TargetServer: b.server,
		Records:      b.infra.Records,
	}
	for _, podResult := range results {
		result.QSent += podResult.QSent
		result.QCompleted += podResult.QCompleted
		result.QLost += podResult.QLost
		result.QInterrupted += podResult.QInterrupted
		result.ResponseCodes += podResult.ResponseCodes
		result.QPS += podResult.QPS
		result.AvgLatency += podResult.AvgLatency
		result.MaxLatency += podResult.MaxLatency
		result.MinLatency += podResult.MinLatency
		result.LatencyStdev += podResult.LatencyStdev
	}
	result.QPS = roundFloat(result.QPS)
	result.AvgLatency /= float64(len(results))
	result.AvgLatency = roundFloat(result.AvgLatency)
	result.MaxLatency /= float64(len(results))
	result.MaxLatency = roundFloat(result.MaxLatency)
	result.MinLatency /= float64(len(results))
	result.MinLatency = roundFloat(result.MinLatency)
	result.LatencyStdev /= float64(len(results))
	result.LatencyStdev = roundFloat(result.LatencyStdev)
	return result
}

func roundFloat(n float64) float64 {
	return math.Floor(n*math.Pow(10, 2)) / math.Pow(10, 2)
}
