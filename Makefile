# Image URL to use all building/pushing image targets
TAG ?= $(shell git describe --abbrev=7)
IMG ?= us-central1-docker.pkg.dev/dev-artifact-registry/docker-images/concourse-oci-helm-chart-resource:$(TAG)

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
