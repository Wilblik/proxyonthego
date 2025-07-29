BIN_DIR := bin
TEST_DIR := perf_test

CMD_DIRS := $(wildcard cmd/*)
CMDS := $(notdir $(CMD_DIRS))

TARGETS := $(addprefix $(BIN_DIR)/, $(CMDS))

.DEFAULT_GOAL := all

all: build

build: $(TARGETS)

$(BIN_DIR)/%: cmd/%
	@echo "Building $*..."
	@mkdir -p $(BIN_DIR)
	@go build -o $@ ./cmd/$*

$(CMDS): %: $(BIN_DIR)/%

test: build
	@echo "Running performance test suite..."
	@./perf_test.sh

clean:
	@echo "Cleaning up project..."
	@rm -rf $(BIN_DIR) $(TEST_DIR)

.PHONY: all build test clean
