package share

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/ipfs/go-blockservice"
	format "github.com/ipfs/go-ipld-format"

	"github.com/celestiaorg/celestia-node/ipld"
)

type LAOption func(*LightAvailability)

// To be used with the construction
// example:
//
// NewLightvailability(
//
//	bServ,
//	disc,
//	WithTimeout(10 * time.Minute),
//
// )
func WithLightAvailabilityTimeout(timeout time.Duration) LAOption {
	return func(la *LightAvailability) {
		la.timeout = timeout
	}
}

// Use for default timeout value
func WithLightAvailabilityTimeoutDefault() LAOption {
	return func(la *LightAvailability) {
		la.timeout = DefaultAvailabilityTimeout
	}
}

// DefaultSampleAmount sets the default amount of samples to be sampled from the network by lightAvailability.
var DefaultSampleAmount = 16

func WithSampleAmount(sampleAmount int) LAOption {
	return func(la *LightAvailability) {
		la.sampleAmount = sampleAmount
	}
}

func WithDefaultSampleAmount() LAOption {
	return func(la *LightAvailability) {
		la.sampleAmount = DefaultSampleAmount
	}
}

// LightAvailability implements Availability using Data Availability Sampling technique.
// It is light because it does not require the downloading of all the data to verify
// its availability. It is assumed that there are a lot of lightAvailability instances
// on the network doing sampling over the same Root to collectively verify its availability.
type LightAvailability struct {
	// configuration parameters
	timeout      time.Duration
	sampleAmount int

	bserv blockservice.BlockService
	// disc discovers new full nodes in the network.
	// it is not allowed to call advertise for light nodes (Full nodes only).
	disc   *Discovery
	cancel context.CancelFunc
}

// NewLightAvailability creates a new light Availability.
func NewLightAvailability(
	bserv blockservice.BlockService,
	disc *Discovery,
	options ...LAOption,
) *LightAvailability {
	la := &LightAvailability{
		bserv: bserv,
		disc:  disc,
	}

	for _, applyOpt := range options {
		applyOpt(la)
	}

	return la
}

func (la *LightAvailability) Start(context.Context) error {
	ctx, cancel := context.WithCancel(context.Background())
	la.cancel = cancel

	go la.disc.ensurePeers(ctx)
	return nil
}

func (la *LightAvailability) Stop(context.Context) error {
	la.cancel()
	return nil
}

// SharesAvailable randomly samples DefaultSamples amount of Shares committed to the given Root.
// This way SharesAvailable subjectively verifies that Shares are available.
func (la *LightAvailability) SharesAvailable(ctx context.Context, dah *Root) error {
	log.Debugw("Validate availability", "root", dah.Hash())
	// We assume the caller of this method has already performed basic validation on the
	// given dah/root. If for some reason this has not happened, the node should panic.
	if err := dah.ValidateBasic(); err != nil {
		log.Errorw("Availability validation cannot be performed on a malformed DataAvailabilityHeader",
			"err", err)
		panic(err)
	}
	samples, err := SampleSquare(len(dah.RowsRoots), la.sampleAmount)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, la.timeout) // timeout is configurable through functional options passed to the constructor
	defer cancel()

	ses := blockservice.NewSession(ctx, la.bserv)
	errs := make(chan error, len(samples))
	for _, s := range samples {
		go func(s Sample) {
			root, leaf := translate(dah, s.Row, s.Col)
			_, err := ipld.GetShare(ctx, ses, root, leaf, len(dah.RowsRoots))
			// we don't really care about Share bodies at this point
			// it also means we now saved the Share in local storage
			select {
			case errs <- err:
			case <-ctx.Done():
			}
		}(s)
	}

	for range samples {
		var err error
		select {
		case err = <-errs:
		case <-ctx.Done():
			err = ctx.Err()
		}

		if err != nil {
			if !errors.Is(err, context.Canceled) {
				log.Errorw("availability validation failed", "root", dah.Hash(), "err", err)
			}
			if format.IsNotFound(err) || errors.Is(err, context.DeadlineExceeded) {
				return ErrNotAvailable
			}

			return err
		}
	}

	return nil
}

// ProbabilityOfAvailability calculates the probability that the
// data square is available based on the amount of samples collected
// (DefaultSampleAmount).
//
// Formula: 1 - (0.75 ** amount of samples)
func (la *LightAvailability) ProbabilityOfAvailability() float64 {
	return 1 - math.Pow(0.75, float64(la.sampleAmount))
}
