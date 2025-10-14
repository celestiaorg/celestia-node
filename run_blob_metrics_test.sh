#!/bin/bash

# Celestia Node Blob Metrics Test
# This script runs a standalone blob metrics test

set -e

echo "🚀 Starting Celestia Node Blob Metrics Test"
echo "==========================================="

# Check if Celestia is built
if [ ! -f "./build/celestia" ]; then
    echo "❌ Celestia binary not found. Building..."
    go build -o ./build/celestia ./cmd/celestia
fi

# Start Grafana and Prometheus
echo "📊 Starting Grafana and Prometheus..."
docker-compose -f docker-compose-grafana.yml up -d

# Wait for services to be ready
echo "⏳ Waiting for services to start..."
sleep 10

echo "✅ Grafana and Prometheus are running"
echo "📈 Grafana: http://localhost:3000 (admin/admin)"
echo "📊 Prometheus: http://localhost:9090"

# Run the standalone test
echo ""
echo "🧪 Running standalone blob metrics test..."
go run test_blob_metrics_standalone.go

echo ""
echo "✅ Test completed!"
echo ""
echo "📊 Metrics are available at:"
echo "   - Prometheus metrics: http://localhost:2121/metrics"
echo "   - Grafana dashboard: http://localhost:3000"
echo "   - Prometheus UI: http://localhost:9090"
echo ""
echo "💡 To stop services: docker-compose -f docker-compose-grafana.yml down"
