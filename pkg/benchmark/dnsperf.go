package benchmark

import (
	"bytes"
	"context"

	"github.com/rs/zerolog/log"

	"regexp"
	"strconv"
	"time"

	"k8s-dnsperf.io/pkg/infra"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/kubectl/pkg/scheme"
)

func NewBenchmark(infra infra.Infra, clients int, server string, timeout, duration time.Duration) Benchmark {
	return Benchmark{
		duration: int(duration.Seconds()),
		clients:  clients,
		timeout:  int(timeout.Seconds()),
		server:   server,
		infra:    infra,
	}
}

// Runs dnsperf benchmark against a list of pods
func (b *Benchmark) Run(podList corev1.PodList, target string) error {
	return nil
}

func (b *Benchmark) exec(ctx context.Context, pod corev1.Pod) (Result, error) {
	var result Result
	cmd := []string{
		"dnsperf",
		"-s",
		b.server,
		"-l",
		string(b.duration),
		"-c",
		string(b.clients),
		"-d",
		"/mnt/input",
	}
	var stdout, stderr bytes.Buffer
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
		Command:   cmd,
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
	return b.parse(stdout.String()), nil
}

/*
Parses dnsperf output and returns Result

Example output:
$ dnsperf  -l 5 -c 10 -d input -c 1 -s 127.0.0.53
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
func (b *Benchmark) parse(stdout string) Result {
	var result Result
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
			log.Fatal().Msgf("Parsing error: %s", res)
			return v
		}
		if len(res) > 0 {
			v, err = strconv.ParseFloat(res[1], 64)
			if err != nil {
				log.Fatal().Msgf("String conversion error error: %s", err)
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
	result.AvgLatency = evalRegex(reAvgLatency, true)
	result.MinLatency = evalRegex(reMinLatency, true)
	result.MaxLatency = evalRegex(reMaxLatency, true)
	result.LatencyStdev = evalRegex(reLatencyStdDev, true)
	// TODO latency-histogram
	return result
}
