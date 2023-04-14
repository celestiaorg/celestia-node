package das

import "time"

// retryStrategy defines a backoff strategy for retries.
type retryStrategy struct {
	backoffIntervals []time.Duration
}

// retry represents a retry attempt with a backoff delay.
type retry struct {
	// count specifies the number of retry attempts made so far.
	count int
	// after specifies the time of the next retry attempt.
	after time.Time
}

// newRetryStrategy creates and initializes a new retry strategy.
func newRetryStrategy(backoffIntervals []time.Duration) retryStrategy {
	return retryStrategy{backoffIntervals: backoffIntervals}
}

// nextRetry creates a retry attempt with a backoff delay based on the retry strategy.
// It takes the number of retry attempts and the time of the last attempt as inputs and returns a
// retry instance and a boolean value indicating whether the retries amount have exceeded.
func (s retryStrategy) nextRetry(tryCount int, lastAttempt time.Time) (retry retry, retriesExceeded bool) {
	retry.count = tryCount

	if len(s.backoffIntervals) == 0 {
		return retry, false
	}

	if tryCount > len(s.backoffIntervals) {
		retry.after = lastAttempt.Add(s.backoffIntervals[len(s.backoffIntervals)-1])
		return retry, true
	}

	retry.after = lastAttempt.Add(s.backoffIntervals[tryCount-1])
	return retry, false
}

// exponentialBackoff generates an array of time.Duration values using an exponential backoff
// strategy.
func exponentialBackoff(baseInterval time.Duration, factor, amount int) []time.Duration {
	backoff := make([]time.Duration, 0, amount)
	next := baseInterval
	for i := 0; i < amount; i++ {
		backoff = append(backoff, next)
		next *= time.Duration(factor)
	}
	return backoff
}
