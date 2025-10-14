#!/bin/bash

echo "🚀 Starting metrics capture test..."

# Start the test in background
cd nodebuilder/tests/tastora
go test -v -tags=integration -run TestBlobMetricsTestSuite/TestBlobSubmissionMetrics > test_output.log 2>&1 &
TEST_PID=$!

echo "⏳ Waiting for test to start..."
sleep 30

# Wait for containers to be ready
echo "🔍 Looking for test containers..."
for i in {1..10}; do
    CONTAINER=$(docker ps --format "table {{.Names}}" | grep TestBlobMetricsTestSuite | head -1)
    if [ ! -z "$CONTAINER" ]; then
        echo "✅ Found container: $CONTAINER"
        break
    fi
    echo "⏳ Waiting for container... ($i/10)"
    sleep 10
done

if [ ! -z "$CONTAINER" ]; then
    echo "⏳ Waiting for blob submissions to complete..."
    sleep 60
    
    echo "📊 Capturing metrics..."
    echo "=========================================="
    echo "🎯 STATE METRICS:"
    echo "=========================================="
    
    # Check for state metrics
    docker exec $CONTAINER wget -qO- http://localhost:8890/metrics | grep "^state_" | head -20
    
    echo ""
    echo "=========================================="
    echo "🔍 SPECIFIC METRICS WE'RE LOOKING FOR:"
    echo "=========================================="
    
    # Check specific metrics
    echo "📈 state_pfb_submission_total:"
    docker exec $CONTAINER wget -qO- http://localhost:8890/metrics | grep "state_pfb_submission_total" || echo "❌ Not found"
    
    echo ""
    echo "📈 state_gas_estimation_total:"
    docker exec $CONTAINER wget -qO- http://localhost:8890/metrics | grep "state_gas_estimation_total" || echo "❌ Not found"
    
    echo ""
    echo "📈 state_pfb_submission_duration_seconds:"
    docker exec $CONTAINER wget -qO- http://localhost:8890/metrics | grep "state_pfb_submission_duration_seconds" || echo "❌ Not found"
    
    echo ""
    echo "=========================================="
    echo "📋 ALL AVAILABLE METRICS (first 30):"
    echo "=========================================="
    docker exec $CONTAINER wget -qO- http://localhost:8890/metrics | grep "^# HELP" | head -30
    
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
else
    echo "❌ No test containers found"
    kill $TEST_PID 2>/dev/null
fi

echo ""
echo "🏁 Metrics capture completed!"
