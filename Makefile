# Image URL to use all building/pushing image targets
TAG ?= $(shell git rev-parse --short HEAD)
IMG ?= ghcr.io/cloudoperators/concourse-oci-helm-chart-resource:$(TAG)

## Tool Binaries
GOIMPORTS ?= $(LOCALBIN)/goimports
GOLINT ?= $(LOCALBIN)/golangci-lint

## Tool Versions
GOLINT_VERSION ?= 2.2.1
GINKGOLINTER_VERSION ?= 0.19.1

## Location to install dependencies an GO binaries
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

.PHONY: all
all: build

##@ Build

.PHONY: build
build: build-check build-in build-out

build-%:
	CGO_ENABLED=0 go build -ldflags '-s -w -extldflags "-static"' -o bin/$* ./cmd/$*/

.PHONY: docker-build
docker-build:
	docker build --platform linux/amd64 -t ${IMG} .

.PHONY: docker-push
docker-push: docker-build
	docker push ${IMG}

.PHONY: goimports
goimports: $(GOIMPORTS)
$(GOIMPORTS): $(LOCALBIN)
	GOBIN=$(LOCALBIN) go install golang.org/x/tools/cmd/goimports@latest

.PHONY: fmt
fmt: goimports
	GOBIN=$(LOCALBIN) go fmt ./...
	$(GOIMPORTS) -w -local github.com/cloudoperators/concourse-oci-helm-chart-resource .

.PHONY: lint
lint: golint
	$(GOLINT) run -v --timeout 5m	

.PHONY: check
check: fmt lint

.PHONY: golint
golint: $(GOLINT)
$(GOLINT): $(LOCALBIN)
	GOBIN=$(LOCALBIN) go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v$(GOLINT_VERSION)
	GOBIN=$(LOCALBIN) go install github.com/nunnatsa/ginkgolinter/cmd/ginkgolinter@v$(GINKGOLINTER_VERSION)
