#!/bin/bash

echo "🚀 Starting comprehensive metrics test with Grafana monitoring..."

# Function to capture metrics
capture_metrics() {
    local container_name=$1
    local test_name=$2
    
    echo "📊 Capturing metrics from $container_name..."
    echo "=========================================="
    
    # Get all state metrics
    echo "🎯 STATE METRICS:"
    docker exec $container_name wget -qO- http://localhost:8890/metrics | grep "^state_" | head -20
    
    echo ""
    echo "🔍 SPECIFIC METRICS:"
    echo "📈 state_pfb_submission_total:"
    docker exec $container_name wget -qO- http://localhost:8890/metrics | grep "state_pfb_submission_total" || echo "❌ Not found"
    
    echo ""
    echo "📈 state_gas_estimation_total:"
    docker exec $container_name wget -qO- http://localhost:8890/metrics | grep "state_gas_estimation_total" || echo "❌ Not found"
    
    echo ""
    echo "📈 state_pfb_submission_duration_seconds:"
    docker exec $container_name wget -qO- http://localhost:8890/metrics | grep "state_pfb_submission_duration_seconds" | grep -v "bucket" | head -3 || echo "❌ Not found"
    
    echo ""
    echo "=========================================="
}

# Start the test in background
cd nodebuilder/tests/tastora
echo "⏳ Starting tastora test..."
go test -v -tags=integration -run TestBlobMetricsTestSuite/TestBlobSubmissionMetrics > test_output.log 2>&1 &
TEST_PID=$!

echo "⏳ Waiting for test to start and containers to be ready..."
sleep 45

# Wait for containers to be ready
echo "🔍 Looking for test containers..."
for i in {1..15}; do
    CONTAINER=$(docker ps --format "table {{.Names}}" | grep TestBlobMetricsTestSuite | head -1)
    if [ ! -z "$CONTAINER" ]; then
        echo "✅ Found container: $CONTAINER"
        break
    fi
    echo "⏳ Waiting for container... ($i/15)"
    sleep 10
done

if [ ! -z "$CONTAINER" ]; then
    echo "⏳ Waiting for blob submissions to start..."
    sleep 30
    
    echo "📊 CAPTURING METRICS DURING TEST EXECUTION:"
    echo "=========================================="
    capture_metrics $CONTAINER "during_execution"
    
    echo ""
    echo "⏳ Waiting for more blob submissions..."
    sleep 30
    
    echo "📊 CAPTURING METRICS AFTER MORE SUBMISSIONS:"
    echo "=========================================="
    capture_metrics $CONTAINER "after_submissions"
    
    # Wait for test to complete
    echo ""
    echo "⏳ Waiting for test to complete..."
    wait $TEST_PID
    echo "✅ Test completed!"
    
    # Show test results
    echo ""
    echo "=========================================="
    echo "📄 TEST RESULTS:"
    echo "=========================================="
    tail -20 test_output.log
    
    echo ""
    echo "🎯 MONITORING ENDPOINTS:"
    echo "=========================================="
    echo "📊 Prometheus: http://localhost:9090"
    echo "📈 Grafana: http://localhost:3000 (admin/admin)"
    echo "🔍 Celestia Metrics: http://localhost:8890/metrics"
    echo ""
    echo "💡 You can now:"
    echo "   1. Open Grafana at http://localhost:3000"
    echo "   2. Login with admin/admin"
    echo "   3. View the Celestia metrics dashboard"
    echo "   4. Check Prometheus at http://localhost:9090 for raw metrics"
    
else
    echo "❌ No test containers found"
    kill $TEST_PID 2>/dev/null
fi

echo ""
echo "🏁 Comprehensive metrics test completed!"
