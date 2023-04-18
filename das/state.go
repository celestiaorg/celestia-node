package das

import (
	"context"
	"sync/atomic"

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
	// stores heights of failed headers with amount of retry attempt as value
	failed map[uint64]int
	// inRetry stores (height -> attempt count) of failed headers that are currently being retried by
	// workers
	inRetry map[uint64]int

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

// newCoordinatorState initiates state for samplingCoordinator
func newCoordinatorState(params Parameters) coordinatorState {
	return coordinatorState{
		sampleFrom:    params.SampleFrom,
		samplingRange: params.SamplingRange,
		inProgress:    make(map[int]func() workerState),
		failed:        make(map[uint64]int),
		inRetry:       make(map[uint64]int),
		nextJobID:     0,
		next:          params.SampleFrom,
		networkHead:   params.SampleFrom,
		catchUpDoneCh: make(chan struct{}),
	}
}

func (s *coordinatorState) resumeFromCheckpoint(c checkpoint) {
	s.next = c.SampleFrom
	s.networkHead = c.NetworkHead

	for h := range c.Failed {
		// TODO(@walldiss): reset retry counter to allow retries after restart. Will be removed when retry
		// backoff is implemented.
		s.failed[h] = 0
	}
}

func (s *coordinatorState) handleResult(res result) {
	delete(s.inProgress, res.id)

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
		failCount := 1
		if res.job.jobType == retryJob {
			// if job was already in retry and failed again, persist attempt count
			failCount += s.inRetry[h]
			delete(s.inRetry, h)
		}

		s.failed[h] = failCount
	}
	s.checkDone()
}

func (s *coordinatorState) isNewHead(newHead int64) bool {
	// seen this header before
	if uint64(newHead) <= s.networkHead {
		log.Warnf("received head height: %v, which is lower or the same as previously known: %v", newHead, s.networkHead)
		return false
	}
	return true
}

func (s *coordinatorState) updateHead(newHead int64) {
	if s.networkHead == s.sampleFrom {
		log.Infow("found first header, starting sampling")
	}

	s.networkHead = uint64(newHead)
	log.Debugw("updated head", "from_height", s.networkHead, "to_height", newHead)
	s.checkDone()
}

// recentJob creates a job to process a recent header.
func (s *coordinatorState) recentJob(header *header.ExtendedHeader) job {
	height := uint64(header.Height())
	// move next, to prevent catchup job from processing same height
	if s.next == height {
		s.next++
	}
	s.nextJobID++
	return job{
		id:      s.nextJobID,
		jobType: recentJob,
		header:  header,
		from:    height,
		to:      height,
	}
}

// nextJob will return next catchup or retry job according to priority (catchup > retry)
func (s *coordinatorState) nextJob() (next job, found bool) {
	// check for catchup job
	if job, found := s.catchupJob(); found {
		return job, found
	}

	// if caught up already, make a retry job
	return s.retryJob()
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
	for h, count := range s.failed {
		// TODO(@walldiss): limit max amount of retries until retry backoff is implemented
		if count > 3 {
			continue
		}

		// move header from failed into retry
		delete(s.failed, h)
		s.inRetry[h] = count
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
	for h, count := range s.failed {
		failed[h] += count
		if h < lowestFailedOrInProgress {
			lowestFailedOrInProgress = h
		}
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
