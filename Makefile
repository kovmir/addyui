build:
	go build -o addyui ./cmd/addyui

install:
	go install ./cmd/addyui

test:
	go test ./...

demo: build
	go build -o mockaddy ./cmd/mockaddy
	./mockaddy -addr 127.0.0.1:18099 & \
	mock_pid=$$!; \
	trap 'kill $$mock_pid 2>/dev/null' EXIT; \
	sleep 0.3; \
	ADDY_BASE_URL=http://127.0.0.1:18099 ADDY_API_KEY=demo vhs vhs/demo.tape

.PHONY: build install demo
