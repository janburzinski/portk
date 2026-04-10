APP_NAME := portk
CMD_PATH := ./cmd/portk
BIN_DIR := bin
BIN_PATH := $(BIN_DIR)/$(APP_NAME)

.PHONY: help run dev-all build install test fmt clean

help:
	@echo "Available targets:"
	@echo "  make run      - Run app (TUI mode, dev processes only)"
	@echo "  make dev-all  - Run app (TUI mode, all listening ports)"
	@echo "  make build    - Build binary to $(BIN_PATH)"
	@echo "  make install  - Install binary to GOPATH/bin"
	@echo "  make test     - Run all tests"
	@echo "  make fmt      - Format all Go files"
	@echo "  make clean    - Remove build artifacts"

run:
	go run $(CMD_PATH)

dev-all:
	go run $(CMD_PATH) --all

build:
	@mkdir -p $(BIN_DIR)
	go build -o $(BIN_PATH) $(CMD_PATH)

install:
	go install $(CMD_PATH)

test:
	go test ./...

fmt:
	gofmt -w $$(find . -name '*.go' -not -path './opensrc/*')

clean:
	rm -rf $(BIN_DIR)
