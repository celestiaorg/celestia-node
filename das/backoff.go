package das

import "time"

// retryStrategy defines a backoff for retries.
type retryStrategy struct {
	// attempts delays will follow durations stored in retryIntervals
	retryIntervals []time.Duration
}

// newRetryStrategy creates and initializes a new retry backoff.
func newRetryStrategy(retryIntervals []time.Duration) retryStrategy {
	return retryStrategy{retryIntervals: retryIntervals}
}

// nextRetry creates a retry attempt with a backoff delay based on the retry backoff.
// It takes the number of retry attempts and the time of the last attempt as inputs and returns a
// retry instance and a boolean value indicating whether the retries amount have exceeded.
func (s retryStrategy) nextRetry(lastRetry retry, lastAttempt time.Time) (retry retry, retriesExceeded bool) {
	lastRetry.count++

	if len(s.retryIntervals) == 0 {
		return lastRetry, false
	}

	if lastRetry.count > len(s.retryIntervals) {
		// try count exceeded backoff try limit
		retry.after = lastAttempt.Add(s.retryIntervals[len(s.retryIntervals)-1])
		return lastRetry, true
	}

	lastRetry.after = lastAttempt.Add(s.retryIntervals[lastRetry.count-1])
	return lastRetry, false
}

// exponentialBackoff generates an array of time.Duration values using an exponential growth
// multiplier.
func exponentialBackoff(baseInterval time.Duration, multiplier, amount int) []time.Duration {
	backoff := make([]time.Duration, 0, amount)
	next := baseInterval
	for i := 0; i < amount; i++ {
		backoff = append(backoff, next)
		next *= time.Duration(multiplier)
	}
	return backoff
}
