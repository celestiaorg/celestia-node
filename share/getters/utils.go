package getters

import (
	"context"
	"errors"
	"time"

	logging "github.com/ipfs/go-log/v2"
	"go.opentelemetry.io/otel"
)

var (
	tracer = otel.Tracer("share/getters")
	log    = logging.Logger("share/getters")

	errOperationNotSupported = errors.New("operation is not supported")
)

// ctxWithSplitTimeout will split timeout stored in context by splitFactor and return the result if
// it is greater than minTimeout. minTimeout == 0 will be ignored, splitFactor <= 0 will be ignored
func ctxWithSplitTimeout(
	ctx context.Context,
	splitFactor int,
	minTimeout time.Duration,
) (context.Context, context.CancelFunc) {
	deadline, ok := ctx.Deadline()
	if !ok || splitFactor <= 0 {
		if minTimeout == 0 {
			return context.WithCancel(ctx)
		}
		return context.WithTimeout(ctx, minTimeout)
	}

	timeout := time.Until(deadline)
	if timeout < minTimeout {
		return context.WithCancel(ctx)
	}

	splitTimeout := timeout / time.Duration(splitFactor)
	if splitTimeout < minTimeout {
		return context.WithTimeout(ctx, minTimeout)
	}
	return context.WithTimeout(ctx, splitTimeout)
}

// ErrorContains reports whether any error in err's tree matches any error in targets tree.
func ErrorContains(err, target error) bool {
	if errors.Is(err, target) || target == nil {
		return true
	}

	target = errors.Unwrap(target)
	if target == nil {
		return false
	}
	return ErrorContains(err, target)
}
