#!/bin/bash

# A script to benchmark the proxy's performance against a direct connection.
# Requires hey (https://github.com/rakyll/hey)

if ! command -v hey &> /dev/null
then
    echo "'hey' command could not be found."
    echo "Please install it by running: go install github.com/rakyll/hey@latest"
    exit 1
fi

TOTAL_REQUESTS=2000
CONCURRENCY=100

TEST_SERVER_ADDR="localhost:9090"
TEST_SERVER_2_ADDR="localhost:9091"
PROXY_ADDR="localhost:8080"
REVERSE_PROXY_ADDR="localhost:8081"

TEST_DIR="perf_test"
PROXY_CMD="./bin/proxy"
REVERSE_PROXY_CMD="./bin/reverse_proxy"
TEST_SERVER_CMD="./bin/test_server"

echo "Ensuring binaries exist..."
if [ ! -f $PROXY_CMD ] || [ ! -f $REVERSE_PROXY_CMD ] || [ ! -f $TEST_SERVER_CMD ]; then
    echo "No binaries found. Run 'make test' or build binaries first."
    exit 1
fi

echo "Ensuring test directory '${TEST_DIR}' exists..."
mkdir -p $TEST_DIR

cleanup() {
  echo "Cleaning up background processes..."
  kill $PROXY_PID &> /dev/null || true
  kill $REVERSE_PROXY_PID &> /dev/null || true
  kill $TEST_SERVER_PID &> /dev/null || true
  kill $TEST_SERVER_2_PID &> /dev/null || true
  echo "Cleanup complete."
}

trap cleanup EXIT

echo "Starting the test server on $TEST_SERVER_ADDR"
$TEST_SERVER_CMD --quiet --port 9090 &
TEST_SERVER_PID=$!

echo "Starting the test server 2 on $TEST_SERVER_2_ADDR"
$TEST_SERVER_CMD --quiet --port 9091 &
TEST_SERVER_2_PID=$!

echo "Starting the proxy server on $PROXY_ADDR"
$PROXY_CMD --quiet --port 8080 &
PROXY_PID=$!

echo "Starting the reverse proxy server on $REVERSE_PROXY_ADDR"
$REVERSE_PROXY_CMD --quiet --config reverse_proxy_conf.yaml &
REVERSE_PROXY_PID=$!

echo "Waiting for servers to initialize..."
sleep 3

echo "Verifying server are running..."
if ! kill -0 $TEST_SERVER_PID &> /dev/null; then
    echo "Test server failed to start. Exiting."
    exit 1
fi
if ! kill -0 $TEST_SERVER_2_PID &> /dev/null; then
    echo "Test server failed to start. Exiting."
    exit 1
fi
if ! kill -0 $PROXY_PID &> /dev/null; then
    echo "Proxy server failed to start. Exiting."
    exit 1
fi
if ! kill -0 $REVERSE_PROXY_PID &> /dev/null; then
    echo "Reverse proxy server failed to start. Exiting."
    exit 1
fi

echo "Running benchmark DIRECTLY to the test server 1..."
hey -n $TOTAL_REQUESTS -c $CONCURRENCY http://$TEST_SERVER_ADDR > $TEST_DIR/direct_results.txt
echo "Direct benchmark complete. Results saved to $TEST_DIR/direct_results.txt"

echo "Running benchmark DIRECTLY to the test server 2..."
hey -n $TOTAL_REQUESTS -c $CONCURRENCY http://$TEST_SERVER_2_ADDR > $TEST_DIR/direct_results_2.txt
echo "Direct benchmark complete. Results saved to $TEST_DIR/direct_results_2.txt"

echo "Running benchmark THROUGH THE PROXY to the test server..."
hey -n $TOTAL_REQUESTS -c $CONCURRENCY -x http://$PROXY_ADDR http://$TEST_SERVER_ADDR > $TEST_DIR/proxy_results.txt
echo "Proxy benchmark complete. Results saved to $TEST_DIR/proxy_results.txt"

echo "Running benchmark THROUGH THE REVERSE PROXY to both test servers..."
hey -n $((TOTAL_REQUESTS * 2)) -c $CONCURRENCY http://$REVERSE_PROXY_ADDR > $TEST_DIR/reverse_proxy_results.txt
echo "Reverse proxy benchmark complete. Results saved to $TEST_DIR/reverse_proxy_results.txt"

echo "All tests finished."
