CONTAINER_ENGINE ?= $(shell command -v podman 2>/dev/null || command -v docker 2>/dev/null || echo "docker")
IMAGE_NAME ?= dep-impact-analysis-action
IMAGE_TAG ?= latest

.PHONY: build test lint image clean

build:
	CGO_ENABLED=0 go build -ldflags="-s -w" -o bin/deptriage ./cmd/deptriage/

test:
	go test -race -coverprofile=coverage.out ./...

lint:
	golangci-lint run ./...

image:
	$(CONTAINER_ENGINE) build -f Containerfile -t $(IMAGE_NAME):$(IMAGE_TAG) .

clean:
	rm -rf bin/ coverage.out
