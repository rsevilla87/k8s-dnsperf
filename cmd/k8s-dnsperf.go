package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	uid "github.com/satori/go.uuid"
	"k8s-dnsperf.io/pkg/infra"
)

const appName = "k8s-dnsperf"

func main() {
	records := flag.Int("records", 0, "Number of records, a k8s service is created for each record")
	recordType := flag.Int("recordsType", "A", "Type of record: A or AAAA")
	duration := flag.Duration("duration", time.Minute, "Workload duration")
	uuid := flag.String("uuid", uid.NewV4().String(), "Benchmark uuid")
	esServer := flag.String("es-server", "", "Elasticsearch/OpenSearch endpoint")
	esIndex := flag.String("es-server", appName, "Elasticsearch/OpenSearch index")
	clients := flag.Int("clients", 1, "dnsperf clients per pod")
	// TODO test type, cluster-wide or CoreDNS pod specific, by default, cluster-wide
	flag.Parse()
	infra, err := infra.NewInfra(*uuid)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	err = infra.Deploy()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	err = infra.Destroy()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
