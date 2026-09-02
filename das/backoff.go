package das

import (
	"math/rand/v2"
	"time"
)

var (
	// defaultBackoffInitialInterval is the upper bound for the first retry delay
	defaultBackoffInitialInterval = time.Minute
	// each retry delay upper bound grows by defaultBackoffMultiplier
	defaultBackoffMultiplier = 4
	// after defaultBackoffMaxRetryCount attempts the retry delay upper bound will stop growing
	// and each retry attempt will produce WARN log
	defaultBackoffMaxRetryCount = 4
)

// retryStrategy defines a backoff for retries.
type retryStrategy struct {
	// retryIntervals stores the upper bound for each retry delay
	retryIntervals []time.Duration
}

// newRetryStrategy creates and initializes a new retry backoff.
func newRetryStrategy(retryIntervals []time.Duration) retryStrategy {
	return retryStrategy{retryIntervals: retryIntervals}
}

// nextRetry creates a retry attempt with a backoff delay based on the retry backoff.
// It takes the number of retry attempts and the time of the last attempt as inputs and returns a
// retry instance and a boolean value indicating whether the retries amount have exceeded.
func (s retryStrategy) nextRetry(lastRetry retryAttempt, lastAttempt time.Time,
) (retry retryAttempt, retriesExceeded bool) {
	lastRetry.count++

	if len(s.retryIntervals) == 0 {
		return lastRetry, false
	}

	if lastRetry.count > len(s.retryIntervals) {
		// try count exceeded backoff try limit
		lastRetry.after = lastAttempt.Add(rand.N(s.retryIntervals[len(s.retryIntervals)-1])) //nolint:gosec
		return lastRetry, true
	}

	lastRetry.after = lastAttempt.Add(rand.N(s.retryIntervals[lastRetry.count-1])) //nolint:gosec
	return lastRetry, false
}

// exponentialBackoff generates an array of time.Duration values using an exponential growth
// multiplier.
func exponentialBackoff(baseInterval time.Duration, multiplier, amount int) []time.Duration {
	backoff := make([]time.Duration, 0, amount)
	next := baseInterval
	for range amount {
		backoff = append(backoff, next)
		next *= time.Duration(multiplier)
	}
	return backoff
}
