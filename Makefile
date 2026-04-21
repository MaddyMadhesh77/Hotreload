# Variables
BINARY     := ./bin/hotreload
TESTSERVER := ./testserver
SERVER_BIN := ./bin/testserver

.PHONY: all build clean test demo run-demo

all: build

## build: Compile the hotreload binary.
build:
	@echo "==> Building hotreload..."
	@mkdir -p ./bin
	go build -o $(BINARY) ./cmd/hotreload
	@echo "==> Binary: $(BINARY)"

## build-server: Compile the test server binary.
build-server:
	@echo "==> Building testserver..."
	@mkdir -p ./bin
	cd $(TESTSERVER) && go build -o ../bin/testserver .
	@echo "==> Binary: $(SERVER_BIN)"

## test: Run all unit tests.
test:
	@echo "==> Running tests..."
	go test -v -count=1 ./...

## clean: Remove compiled binaries.
clean:
	@echo "==> Cleaning..."
	rm -rf ./bin

## demo: Build hotreload, then launch the demo.
demo: build
	@echo ""
	@echo "==> Starting hotreload demo."
	@echo "    Edit testserver/main.go and save to see hot reload in action."
	@echo "    The server is available at http://localhost:8080"
	@echo ""
	$(BINARY) \
		--root $(TESTSERVER) \
		--run $(TESTSERVER)

## help: Print available Makefile targets.
help:
	@echo ""
	@echo "Available targets:"
	@grep -E '^## ' Makefile | sed 's/## /  /'
	@echo ""
