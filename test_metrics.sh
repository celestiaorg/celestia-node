#!/bin/bash

# Test script to check if state metrics are working
echo "Starting metrics test..."

# Start Prometheus and Grafana
docker-compose -f docker-compose-grafana.yml up -d

# Wait for services to be ready
sleep 10

# Run a simple test to check if the metrics are working
echo "Running test to check metrics..."

# Start the test in background
cd nodebuilder/tests/tastora
go test -v -tags=integration -run TestBlobMetricsTestSuite/TestBlobSubmissionMetrics &
TEST_PID=$!

# Wait for the test to start
sleep 30

# Check if any containers are running
CONTAINERS=$(docker ps --format "table {{.Names}}" | grep TestBlobMetricsTestSuite | head -1)

if [ ! -z "$CONTAINERS" ]; then
    echo "Found running container: $CONTAINERS"
    
    # Wait a bit more for metrics to be available
    sleep 20
    
    # Check for state metrics
    echo "Checking for state metrics..."
    docker exec $CONTAINERS wget -qO- http://localhost:8890/metrics | grep "state_pfb_submission_total" || echo "state_pfb_submission_total not found"
    docker exec $CONTAINERS wget -qO- http://localhost:8890/metrics | grep "state_gas_estimation_total" || echo "state_gas_estimation_total not found"
    
    # Show all state metrics
    echo "All state metrics:"
    docker exec $CONTAINERS wget -qO- http://localhost:8890/metrics | grep "^state_"
    
    # Wait for test to complete
    wait $TEST_PID
else
    echo "No test containers found"
    kill $TEST_PID 2>/dev/null
fi

echo "Test completed"
