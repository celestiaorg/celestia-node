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
