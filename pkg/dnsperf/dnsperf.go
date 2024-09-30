package dnsperf

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
)

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
func Parse(stdout string) Result {
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
			fmt.Printf("Parsing error: %s\n", res)
			os.Exit(1)
		}
		if len(res) > 0 {
			v, err = strconv.ParseFloat(res[1], 64)
			if err != nil {
				fmt.Printf("String conversion error error: %s\n", err)
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
