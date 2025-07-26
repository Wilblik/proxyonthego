#!/bin/bash

# A script to benchmark the proxy's performance against a direct connection.
# Requires hey (https://github.com/rakyll/hey)

if ! command -v hey &> /dev/null
then
    echo "'hey' command could not be found."
    echo "Please install it by running: go install github.com/rakyll/hey@latest"
    exit 1
fi

TEST_SERVER_ADDR="localhost:9090"
PROXY_ADDR="localhost:8080"
TOTAL_REQUESTS=2000
CONCURRENCY=100
TEST_DIR="perf_test"
PROXY_CMD="./bin/proxy"
TEST_SERVER_CMD="./bin/test_server"

echo "Ensuring binaries exist..."
if [ ! -f $PROXY_CMD ] || [ ! -f $TEST_SERVER_CMD ]; then
    echo "No binaries found. Run 'make test' or build binaries first."
    exit 1
fi

echo "Ensuring test directory '${TEST_DIR}' exists..."
mkdir -p $TEST_DIR

cleanup() {
  echo "Cleaning up background processes..."
  kill $PROXY_PID &> /dev/null || true
  kill $TEST_SERVER_PID &> /dev/null || true
  echo "Cleanup complete."
}

trap cleanup EXIT

echo "Starting the test server on $TEST_SERVER_ADDR"
$TEST_SERVER_CMD --port 9090 &
TEST_SERVER_PID=$!

echo "Starting the proxy server on $PROXY_ADDR"
$PROXY_CMD --port 8080 --quiet &
PROXY_PID=$!

echo "Waiting for servers to initialize..."
sleep 3

echo "Verifying server are running..."
if ! kill -0 $TEST_SERVER_PID &> /dev/null; then
    echo "Test server failed to start. Exiting."
    exit 1
fi
if ! kill -0 $PROXY_PID &> /dev/null; then
    echo "Proxy server failed to start. Exiting."
    exit 1
fi

echo "Running benchmark DIRECTLY to the test server..."
hey -n $TOTAL_REQUESTS -c $CONCURRENCY http://$TEST_SERVER_ADDR > $TEST_DIR/direct_results.txt
echo "Direct benchmark complete. Results saved to $TEST_DIR/direct_results.txt"

echo "Running benchmark THROUGH THE PROXY to the test server..."
hey -n $TOTAL_REQUESTS -c $CONCURRENCY -x http://$PROXY_ADDR http://$TEST_SERVER_ADDR > $TEST_DIR/proxy_results.txt
echo "Proxy benchmark complete. Results saved to $TEST_DIR/proxy_results.txt"

echo "All tests finished."
