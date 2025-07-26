BIN_DIR := bin
TEST_DIR := perf_test

PROXY_PKG := ./cmd/proxy
TEST_SERVER_PKG := ./cmd/test_server

PROXY_EXE := $(BIN_DIR)/proxy
TEST_SERVER_EXE := $(BIN_DIR)/test_server

.DEFAULT_GOAL := all

all: build

build: proxy test-server

proxy:
	@echo "Building proxy..."
	@go build -o $(PROXY_EXE) $(PROXY_PKG)

test-server:
	@echo "Building test server..."
	@go build -o $(TEST_SERVER_EXE) $(TEST_SERVER_PKG)

test: build
	@echo "Running performance test suite..."
	@./perf_test.sh

clean:
	@echo "Cleaning up project..."
	@rm -rf $(BIN_DIR) $(TEST_DIR)

.PHONY: all build build-proxy build-test-server test clean
