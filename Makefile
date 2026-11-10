build:
	go build -o addyui ./cmd/addyui

install:
	go install ./cmd/addyui

# Record the README demo GIF: builds the TUI and a mock addy.io API server,
# starts the mock, runs the scripted VHS tape, then stops the mock.
# Requires vhs (github.com/charmbracelet/vhs) on PATH.
demo: build
	go build -o mockaddy ./cmd/mockaddy
	./mockaddy -addr 127.0.0.1:18099 & \
	mock_pid=$$!; \
	trap 'kill $$mock_pid 2>/dev/null' EXIT; \
	sleep 0.3; \
	ADDY_BASE_URL=http://127.0.0.1:18099 ADDY_API_KEY=demo vhs vhs/demo.tape

.PHONY: build install demo
