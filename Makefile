GO ?= go
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS = -X main.version=$(VERSION)

build:
	$(GO) build -ldflags "$(LDFLAGS)" -o addyui ./cmd/addyui

install:
	$(GO) install ./cmd/addyui

test:
	$(GO) test -race ./...

vet:
	$(GO) vet ./...

fmt:
	gofmt -l .

check: test vet

clean:
	rm -f addyui mockaddy

.PHONY: build install test vet fmt check clean
