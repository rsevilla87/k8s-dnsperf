package main

import (
	"encoding/json"
	"flag"
	"os"
	"time"

	"github.com/cloud-bulldozer/go-commons/indexers"
	"github.com/cloud-bulldozer/go-commons/version"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	uid "github.com/satori/go.uuid"
	"k8s-dnsperf.io/pkg/benchmark"
	"k8s-dnsperf.io/pkg/infra"
)

const appName = "k8s-dnsperf"

func main() {
	var recordType infra.RecordType
	var indexer indexers.Indexer
	uuid := flag.String("uuid", uid.NewV4().String(), "Benchmark uuid")
	selector := flag.String("selector", "node-role.kubernetes.io/worker=", "DaemonSet node Selector")
	records := flag.Int("records", 1, "Number of records, each records represents a k8s service")
	recordTypeString := flag.String("recordsType", "A", "Type of record: A or AAAA")
	duration := flag.Duration("duration", time.Minute, "Workload duration")
	timeout := flag.Duration("timeout", time.Second, "dnsperf timeout")
	server := flag.String("dns-server", "172.30.0.10", "DNS server to load")
	port := flag.Int("port", 53, "DNS server port")
	logLevel := flag.String("loglevel", "info", "Log level, valid values are trace, debug, info, warn, error")
	esServer := flag.String("es-server", "", "Elasticsearch/OpenSearch endpoint")
	esIndex := flag.String("es-index", appName, "Elasticsearch/OpenSearch index")
	clients := flag.Int("clients", 1, "dnsperf clients per pod")
	flag.Parse()
	level, err := zerolog.ParseLevel(*logLevel)
	if err != nil {
		log.Fatal().Msg(err.Error())
	}
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: "2006-01-02 15:04:05"}).Level(level).With().Caller().Logger()
	log.Info().Msgf("Starting %s %s@%s", appName, version.Version, version.GitCommit)
	if *esServer != "" {
		log.Info().Msg("Creating indexer 💾")
		indexer, err = indexers.NewOpenSearchIndexer(indexers.IndexerConfig{
			Type:               indexers.OpenSearchIndexer,
			Servers:            []string{*esServer},
			Index:              *esIndex,
			InsecureSkipVerify: true,
		})
		if err != nil {
			log.Fatal().Msgf("Error creating indexer: %s", err.Error())
		}
	}
	switch *recordTypeString {
	case "A":
		recordType = infra.RecordA
	case "AAAA":
		recordType = infra.RecordAAAA
	default:
		log.Fatal().Msgf("Invalid record type: %s", recordType)
	}
	infra, err := infra.NewInfra(*uuid, *selector, *records, recordType)
	if err != nil {
		log.Fatal().Msg(err.Error())
	}
	err = infra.Deploy()
	if err != nil {
		log.Fatal().Msg(err.Error())
	}
	defer func() {
		if err := infra.Destroy(); err != nil {
			log.Fatal().Msg(err.Error())
		}
	}()
	b := benchmark.NewBenchmark(infra, *clients, *server, *port, *timeout, *duration)
	benchmarkResult, err := b.Run()
	if err != nil {
		log.Fatal().Msg(err.Error())
	}
	if indexer != nil {
		err = indexResults(indexer, benchmarkResult)
		if err != nil {
			log.Fatal().Msg(err.Error())
		}
	}
	j, _ := json.MarshalIndent(benchmarkResult, "", "  ")
	log.Debug().Msg(string(j))
}

func indexResults(indexer indexers.Indexer, benchmarkResult benchmark.Result) error {
	log.Info().Msg("Indexing results")
	var benchmarkResultDoc []any
	benchmarkResultDoc = append(benchmarkResultDoc, benchmarkResult)
	output, err := indexer.Index(benchmarkResultDoc, indexers.IndexingOpts{})
	if err != nil {
		return err
	}
	log.Info().Msg(output)
	return nil
}
