CONTAINER_BUILD ?= podman build
CONTAINER_NS ?= quay.io/cloud-bulldozer
GIT_COMMIT = $(shell git rev-parse HEAD)

BRANCH := $(shell git rev-parse --abbrev-ref HEAD)
ifeq ($(BRANCH),HEAD)
	VERSION := $(shell git describe --tags --abbrev=0)
else
	VERSION := $(BRANCH)
endif

BUILD_DATE = $(shell date '+%Y-%m-%d-%H:%M:%S')
K8S_DNSPERF_VERSION = github.com/cloud-bulldozer/go-commons/version

BIN_DIR = bin
BIN_NAME = k8s-dnsperf
BIN_PATH = $(BIN_DIR)/$(BIN_NAME)
SOURCES = $(shell find . -type f -name "*.go")
CGO = 0

.PHONY: build lint clean

all: lint build container-build

build: $(BIN_PATH)

$(BIN_PATH): $(SOURCES)
	GOARCH=$(shell go env GOARCH) CGO_ENABLED=$(CGO) go build -v -ldflags "-X $(K8S_DNSPERF_VERSION).GitCommit=$(GIT_COMMIT) -X $(K8S_DNSPERF_VERSION).Version=$(VERSION) -X $(K8S_DNSPERF_VERSION).BuildDate=$(BUILD_DATE)" -o $(BIN_PATH) cmd/k8s-dnsperf.go

container-build:
	@echo "Building the container image"
	$(CONTAINER_BUILD) -f containers/Containerfile -t $(CONTAINER_NS)/$(BIN_NAME) ./containers

gha-build:
	@echo "Building container images"
	$(CONTAINER_BUILD) --jobs=2 -f containers/Containerfile --platform=linux/amd64,linux/arm64,linux/ppc64le ./containers --manifest=$(CONTAINER_NS)/$(BIN_NAME):latest

gha-push: gha-build
	@echo "Publish the Container Images"
	podman manifest push $(CONTAINER_NS)/$(BIN_NAME):latest $(CONTAINER_NS)/$(BIN_NAME):latest

clean:
	rm -Rf $(BIN_DIR)

lint:
	golangci-lint run
