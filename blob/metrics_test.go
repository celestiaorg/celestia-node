package blob

import (
	"context"
	"testing"
	"time"
)

func TestMetricsMethodsExist(t *testing.T) {
	// Test that our metrics methods exist and can be called with nil metrics
	var metrics *metrics = nil

	ctx := context.Background()

	// These should not panic when metrics is nil
	metrics.ObserveRetrieval(ctx, 200*time.Millisecond, nil)
	metrics.ObserveProof(ctx, 300*time.Millisecond, nil)

	t.Log("✅ All metrics methods exist and handle nil metrics correctly")
}

func TestMetricsStruct(t *testing.T) {
	// Test that the metrics struct has all the expected fields
	metrics := &metrics{}

	// Verify the struct has the expected atomic counters
	_ = metrics.totalRetrievals
	_ = metrics.totalRetrievalErrors
	_ = metrics.totalProofs
	_ = metrics.totalProofErrors

	t.Log("✅ Metrics struct has all expected fields")
}
