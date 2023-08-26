package das

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/celestiaorg/celestia-node/header"
)

// coordinatorState represents the current state of sampling process
type coordinatorState struct {
	// sampleFrom is the height from which the DASer will start sampling
	sampleFrom uint64
	// samplingRange is the maximum amount of headers processed in one job.
	samplingRange uint64

	// keeps track of running workers
	inProgress map[int]func() workerState

	// retryStrategy implements retry backoff
	retryStrategy retryStrategy
	// stores heights of failed headers with amount of retry attempt as value
	failed map[uint64]retryAttempt
	// inRetry stores (height -> attempt count) of failed headers that are currently being retried by
	// workers
	inRetry map[uint64]retryAttempt

	// nextJobID is a unique identifier that will be used for creation of next job
	nextJobID int
	// all headers before next were sent to workers
	next uint64
	// networkHead is the height of the latest known network head
	networkHead uint64

	// catchUpDone indicates if all headers are sampled
	catchUpDone atomic.Bool
	// catchUpDoneCh blocks until all headers are sampled
	catchUpDoneCh chan struct{}
}

// retryAttempt represents a retry attempt with a backoff delay.
type retryAttempt struct {
	// count specifies the number of retry attempts made so far.
	count int
	// after specifies the time for the next retry attempt.
	after time.Time
}

// newCoordinatorState initiates state for samplingCoordinator
func newCoordinatorState(params Parameters) coordinatorState {
	return coordinatorState{
		sampleFrom:    params.SampleFrom,
		samplingRange: params.SamplingRange,
		inProgress:    make(map[int]func() workerState),
		retryStrategy: newRetryStrategy(exponentialBackoff(
			defaultBackoffInitialInterval,
			defaultBackoffMultiplier,
			defaultBackoffMaxRetryCount)),
		failed:        make(map[uint64]retryAttempt),
		inRetry:       make(map[uint64]retryAttempt),
		nextJobID:     0,
		next:          params.SampleFrom,
		networkHead:   params.SampleFrom,
		catchUpDoneCh: make(chan struct{}),
	}
}

func (s *coordinatorState) resumeFromCheckpoint(c checkpoint) {
	s.next = c.SampleFrom
	s.networkHead = c.NetworkHead

	for h, count := range c.Failed {
		// resumed retries should start without backoff delay
		s.failed[h] = retryAttempt{
			count: count,
			after: time.Now(),
		}
	}
}

func (s *coordinatorState) handleResult(res result) {
	delete(s.inProgress, res.id)

	switch res.jobType {
	case recentJob, catchupJob:
		s.handleRecentOrCatchupResult(res)
	case retryJob:
		s.handleRetryResult(res)
	}

	s.checkDone()
}

func (s *coordinatorState) handleRecentOrCatchupResult(res result) {
	// check if the worker retried any of the previously failed heights
	for h := range s.failed {
		if h < res.from || h > res.to {
			continue
		}

		if res.failed[h] == 0 {
			delete(s.failed, h)
		}
	}

	// update failed heights
	for h := range res.failed {
		nextRetry, _ := s.retryStrategy.nextRetry(retryAttempt{}, time.Now())
		s.failed[h] = nextRetry
	}
}

func (s *coordinatorState) handleRetryResult(res result) {
	// move heights that has failed again to failed with keeping retry count, they will be picked up by
	// retry workers later
	for h := range res.failed {
		lastRetry := s.inRetry[h]
		// height will be retried after backoff
		nextRetry, retryExceeded := s.retryStrategy.nextRetry(lastRetry, time.Now())
		if retryExceeded {
			log.Warnw("header exceeded maximum amount of sampling attempts",
				"height", h,
				"attempts", nextRetry.count)
		}
		s.failed[h] = nextRetry
	}

	// processed height are either already moved to failed map or succeeded, cleanup inRetry
	for h := res.from; h <= res.to; h++ {
		delete(s.inRetry, h)
	}
}

func (s *coordinatorState) isNewHead(newHead uint64) bool {
	// seen this header before
	if newHead <= s.networkHead {
		log.Warnf("received head height: %v, which is lower or the same as previously known: %v", newHead, s.networkHead)
		return false
	}
	return true
}

func (s *coordinatorState) updateHead(newHead uint64) {
	if s.networkHead == s.sampleFrom {
		log.Infow("found first header, starting sampling")
	}

	s.networkHead = newHead
	log.Debugw("updated head", "from_height", s.networkHead, "to_height", newHead)
	s.checkDone()
}

// recentJob creates a job to process a recent header.
func (s *coordinatorState) recentJob(header *header.ExtendedHeader) job {
	// move next, to prevent catchup job from processing same height
	if s.next == header.Height() {
		s.next++
	}
	s.nextJobID++
	return job{
		id:      s.nextJobID,
		jobType: recentJob,
		header:  header,
		from:    header.Height(),
		to:      header.Height(),
	}
}

// nextJob will return next catchup or retry job according to priority (retry -> catchup)
func (s *coordinatorState) nextJob() (next job, found bool) {
	// check for if any retry jobs are available
	if job, found := s.retryJob(); found {
		return job, found
	}

	// if no retry jobs, make a catchup job
	return s.catchupJob()
}

// catchupJob creates a catchup job if catchup is not finished
func (s *coordinatorState) catchupJob() (next job, found bool) {
	if s.next > s.networkHead {
		return job{}, false
	}

	to := s.next + s.samplingRange - 1
	if to > s.networkHead {
		to = s.networkHead
	}
	j := s.newJob(catchupJob, s.next, to)
	s.next = to + 1
	return j, true
}

// retryJob creates a job to retry previously failed header
func (s *coordinatorState) retryJob() (next job, found bool) {
	for h, attempt := range s.failed {
		if !attempt.canRetry() {
			// height will be retried later
			continue
		}

		// move header from failed into retry
		delete(s.failed, h)
		s.inRetry[h] = attempt
		j := s.newJob(retryJob, h, h)
		return j, true
	}

	return job{}, false
}

func (s *coordinatorState) putInProgress(jobID int, getState func() workerState) {
	s.inProgress[jobID] = getState
}

func (s *coordinatorState) newJob(jobType jobType, from, to uint64) job {
	s.nextJobID++
	return job{
		id:      s.nextJobID,
		jobType: jobType,
		from:    from,
		to:      to,
	}
}

// unsafeStats collects coordinator stats without thread-safety
func (s *coordinatorState) unsafeStats() SamplingStats {
	workers := make([]WorkerStats, 0, len(s.inProgress))
	lowestFailedOrInProgress := s.next
	failed := make(map[uint64]int)

	// gather worker stats
	for _, getStats := range s.inProgress {
		wstats := getStats()
		var errMsg string
		if wstats.err != nil {
			errMsg = wstats.err.Error()
		}
		workers = append(workers, WorkerStats{
			JobType: wstats.job.jobType,
			Curr:    wstats.curr,
			From:    wstats.from,
			To:      wstats.to,
			ErrMsg:  errMsg,
		})

		for h := range wstats.failed {
			failed[h]++
			if h < lowestFailedOrInProgress {
				lowestFailedOrInProgress = h
			}
		}

		if wstats.curr < lowestFailedOrInProgress {
			lowestFailedOrInProgress = wstats.curr
		}
	}

	// set lowestFailedOrInProgress to minimum failed - 1
	for h, retry := range s.failed {
		failed[h] += retry.count
		if h < lowestFailedOrInProgress {
			lowestFailedOrInProgress = h
		}
	}

	for h, retry := range s.inRetry {
		failed[h] += retry.count
	}

	return SamplingStats{
		SampledChainHead: lowestFailedOrInProgress - 1,
		CatchupHead:      s.next - 1,
		NetworkHead:      s.networkHead,
		Failed:           failed,
		Workers:          workers,
		Concurrency:      len(workers),
		CatchUpDone:      s.catchUpDone.Load(),
		IsRunning:        len(workers) > 0 || s.catchUpDone.Load(),
	}
}

func (s *coordinatorState) checkDone() {
	if len(s.inProgress) == 0 && len(s.failed) == 0 && s.next > s.networkHead {
		if s.catchUpDone.CompareAndSwap(false, true) {
			close(s.catchUpDoneCh)
		}
		return
	}

	if s.catchUpDone.Load() {
		// overwrite channel before storing done flag
		s.catchUpDoneCh = make(chan struct{})
		s.catchUpDone.Store(false)
	}
}

// waitCatchUp waits for sampling process to indicate catchup is done
func (s *coordinatorState) waitCatchUp(ctx context.Context) error {
	if s.catchUpDone.Load() {
		return nil
	}
	select {
	case <-s.catchUpDoneCh:
	case <-ctx.Done():
		return ctx.Err()
	}
	return nil
}

// canRetry returns true if the time stored in the "after" has passed.
func (r retryAttempt) canRetry() bool {
	return r.after.Before(time.Now())
}
