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

demo: build
	$(GO) build -o mockaddy ./cmd/mockaddy
	./mockaddy -addr 127.0.0.1:18099 & \
	mock_pid=$$!; \
	trap 'kill $$mock_pid 2>/dev/null' EXIT; \
	sleep 0.3; \
	ADDY_BASE_URL=http://127.0.0.1:18099 ADDY_API_KEY=demo vhs vhs/demo.tape

.PHONY: build install test vet fmt check clean demo
