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
	var localIndexer indexers.Indexer
	var benchmarkResultDoc []any
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
	localIndexing := flag.Bool("local-indexing", false, "Enable local indexing")
	flag.Parse()
	level, err := zerolog.ParseLevel(*logLevel)
	if err != nil {
		log.Fatal().Msg(err.Error())
	}
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: "2006-01-02 15:04:05"}).Level(level).With().Caller().Logger()
	log.Info().Msgf("Starting %s %s@%s", appName, version.Version, version.GitCommit)
	if *localIndexing {
		log.Info().Msg("Creating local indexer 📁")
		localIndexer, err = indexers.NewLocalIndexer(indexers.IndexerConfig{
			Type:             indexers.LocalIndexer,
			MetricsDirectory: "results",
		})
		if err != nil {
			log.Fatal().Msgf("Error creating indexer: %s", err.Error())
		}

	}
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
	benchmarkResultDoc = append(benchmarkResultDoc, benchmarkResult)
	if indexer != nil {
		err = indexResults(indexer, benchmarkResultDoc, indexers.IndexingOpts{})
		if err != nil {
			log.Fatal().Msg(err.Error())
		}
	}
	if localIndexer != nil {
		err = indexResults(localIndexer, benchmarkResultDoc, indexers.IndexingOpts{MetricName: "k8s-dnsperf"})
		if err != nil {
			log.Fatal().Msg(err.Error())
		}
	}
	j, _ := json.MarshalIndent(benchmarkResult, "", "  ")
	log.Debug().Msg(string(j))
}

func indexResults(indexer indexers.Indexer, benchmarkResultDoc []any, indexingOpts indexers.IndexingOpts) error {
	log.Info().Msg("Indexing results")
	output, err := indexer.Index(benchmarkResultDoc, indexingOpts)
	if err != nil {
		return err
	}
	log.Info().Msg(output)
	return nil
}
