package peers

import (
	"context"
	"sync/atomic"
	"time"
)

// syncPool accumulates peers from shrex.Sub validators and controls message retransmission.
// It will unlock the validator only if there is a proof that header with given datahash exist in a
// chain
type syncPool struct {
	pool *pool

	firstPeer *atomic.Bool

	isValidDataHash    *atomic.Bool
	validatorWaitCh    chan struct{}
	validatorWaitTimer *time.Timer

	isSampled      *atomic.Bool
	waitSamplingCh chan struct{}
}

func newSyncPool() syncPool {
	return syncPool{
		pool:            newPool(),
		firstPeer:       new(atomic.Bool),
		isValidDataHash: new(atomic.Bool),
		isSampled:       new(atomic.Bool),
		waitSamplingCh:  make(chan struct{}),
	}
}

// waitValidation waits for header to sync within timeout.
func (p *syncPool) waitValidation(ctx context.Context) (valid bool) {
	select {
	case <-p.validatorWaitCh:
		return p.isValidDataHash.Load()
	case <-ctx.Done():
		return false
	}
}

func (p *syncPool) waitSampling(ctx context.Context) (sampled bool) {
	select {
	case <-p.waitSamplingCh:
		// block with given datahash got markSampled, allow the pubsub to retransmit the message by
		// returning Accept
		return true
	case <-ctx.Done():
		return false
	}
}

func (p *syncPool) markValidated() {
	if p.isValidDataHash.CompareAndSwap(false, true) {
		// unlock all awaiting Validators.
		// if unable to stop the timer, the channel was already closed by afterfunc
		if p.validatorWaitTimer.Stop() {
			close(p.validatorWaitCh)
		}
	}
}
