package das

import "time"

// retryBackoff defines a backoff backoff for retries.
type retryBackoff struct {
	backoffStrategy []time.Duration
}

// retry represents a retry attempt with a backoff delay.
type retry struct {
	// count specifies the number of retry attempts made so far.
	count int
	// after specifies the time for the next retry attempt.
	after time.Time
}

// newRetryBackOff creates and initializes a new retry backoff.
func newRetryBackOff(backoffStrategy []time.Duration) retryBackoff {
	return retryBackoff{backoffStrategy: backoffStrategy}
}

// nextRetry creates a retry attempt with a backoff delay based on the retry backoff.
// It takes the number of retry attempts and the time of the last attempt as inputs and returns a
// retry instance and a boolean value indicating whether the retries amount have exceeded.
func (s retryBackoff) nextRetry(lastRetry retry, lastAttempt time.Time) (retry retry, retriesExceeded bool) {
	lastRetry.count++

	if len(s.backoffStrategy) == 0 {
		return lastRetry, false
	}

	if lastRetry.count > len(s.backoffStrategy) {
		// try count exceeded backoff backoff try limit
		retry.after = lastAttempt.Add(s.backoffStrategy[len(s.backoffStrategy)-1])
		return lastRetry, true
	}

	lastRetry.after = lastAttempt.Add(s.backoffStrategy[lastRetry.count-1])
	return lastRetry, false
}

// exponentialBackoff generates an array of time.Duration values using an exponential backoff
// backoff.
func exponentialBackoff(baseInterval time.Duration, factor, amount int) []time.Duration {
	backoff := make([]time.Duration, 0, amount)
	next := baseInterval
	for i := 0; i < amount; i++ {
		backoff = append(backoff, next)
		next *= time.Duration(factor)
	}
	return backoff
}
