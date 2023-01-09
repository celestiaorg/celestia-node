package getters

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/multierr"

	"github.com/celestiaorg/celestia-node/share"
)

func TestCascadeGetter(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	const gettersN = 3
	roots := make([]*share.Root, gettersN)
	getters := make([]share.Getter, gettersN)
	for i := range roots {
		getters[i], roots[i] = TestGetter(t)
	}

	getter := NewCascadeGetter(getters, time.Millisecond)
	t.Run("GetShare", func(t *testing.T) {
		for _, r := range roots {
			sh, err := getter.GetShare(ctx, r, 0, 0)
			assert.NoError(t, err)
			assert.NotEmpty(t, sh)
		}
	})

	t.Run("GetEDS", func(t *testing.T) {
		for _, r := range roots {
			sh, err := getter.GetEDS(ctx, r)
			assert.NoError(t, err)
			assert.NotEmpty(t, sh)
		}
	})
}

func TestCascadeSuccessFirst(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	fns := []func(context.Context) (int, error){
		func(context.Context) (int, error) {
			return 1, nil
		},
		func(context.Context) (int, error) {
			return 2, nil
		},
		func(context.Context) (int, error) {
			return 3, nil
		},
	}

	val, err := cascade(ctx, fns, time.Millisecond*10)
	assert.NoError(t, err)
	assert.Equal(t, 1, val)
}

func TestCascadeSuccessSecond(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	fns := []func(context.Context) (int, error){
		func(context.Context) (int, error) {
			return 1, fmt.Errorf("1")
		},
		func(context.Context) (int, error) {
			return 2, nil
		},
		func(context.Context) (int, error) {
			return 3, nil
		},
	}

	val, err := cascade(ctx, fns, time.Millisecond*10)
	assert.NoError(t, err)
	assert.Equal(t, 2, val)
}

func TestCascadeSuccessSecondFirst(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	fns := []func(context.Context) (int, error){
		func(ctx context.Context) (int, error) {
			select {
			case <-time.After(time.Second):
				return 1, nil
			case <-ctx.Done():
				return 0, ctx.Err()
			}
		},
		func(context.Context) (int, error) {
			return 2, nil
		},
		func(context.Context) (int, error) {
			return 3, nil
		},
	}

	val, err := cascade(ctx, fns, time.Millisecond*10)
	assert.NoError(t, err)
	assert.Equal(t, 2, val)
}

func TestCascadeSuccessOnceAllStarted(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	fns := []func(context.Context) (int, error){
		func(ctx context.Context) (int, error) {
			select {
			case <-time.After(time.Millisecond * 3):
				return 1, nil
			case <-ctx.Done():
				return 0, ctx.Err()
			}
		},
		func(context.Context) (int, error) {
			select {
			case <-time.After(time.Millisecond * 4):
				return 2, nil
			case <-ctx.Done():
				return 0, ctx.Err()
			}
		},
		func(context.Context) (int, error) {
			select {
			case <-time.After(time.Millisecond * 5):
				return 3, nil
			case <-ctx.Done():
				return 0, ctx.Err()
			}
		},
	}

	val, err := cascade(ctx, fns, time.Millisecond*1)
	assert.NoError(t, err)
	assert.Equal(t, 1, val)
}

func TestCascadeError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	fns := []func(context.Context) (int, error){
		func(context.Context) (int, error) {
			return 0, fmt.Errorf("1")
		},
		func(context.Context) (int, error) {
			return 0, fmt.Errorf("2")
		},
		func(context.Context) (int, error) {
			return 0, fmt.Errorf("3")
		},
	}

	val, err := cascade(ctx, fns, time.Millisecond*10)
	assert.Error(t, err)
	assert.Len(t, multierr.Errors(err), 3)
	assert.Equal(t, 0, val)
}

func TestCascadeSingle(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	fns := []func(context.Context) (int, error){
		func(ctx context.Context) (int, error) {
			return 1, nil
		},
	}

	val, err := cascade(ctx, fns, time.Second)
	assert.NoError(t, err)
	assert.Equal(t, 1, val)
}
