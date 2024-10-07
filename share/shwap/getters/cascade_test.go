package getters

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/share/shwap"
	"github.com/celestiaorg/celestia-node/share/shwap/getters/mock"
)

func TestCascadeGetter(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	const gettersN = 3
	headers := make([]*header.ExtendedHeader, gettersN)
	getters := make([]shwap.Getter, gettersN)
	for i := range headers {
		getters[i], headers[i] = TestGetter(t)
	}

	getter := NewCascadeGetter(getters)
	t.Run("GetShare", func(t *testing.T) {
		for _, eh := range headers {
			sh, err := getter.GetShare(ctx, eh, 0, 0)
			assert.NoError(t, err)
			assert.NotEmpty(t, sh)
		}
	})

	t.Run("GetEDS", func(t *testing.T) {
		for _, eh := range headers {
			sh, err := getter.GetEDS(ctx, eh)
			assert.NoError(t, err)
			assert.NotEmpty(t, sh)
		}
	})
}

func TestCascade(t *testing.T) {
	ctrl := gomock.NewController(t)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	timeoutGetter := mock.NewMockGetter(ctrl)
	immediateFailGetter := mock.NewMockGetter(ctrl)
	successGetter := mock.NewMockGetter(ctrl)
	ctxGetter := mock.NewMockGetter(ctrl)
	timeoutGetter.EXPECT().GetEDS(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, _ *header.ExtendedHeader) (*rsmt2d.ExtendedDataSquare, error) {
			return nil, context.DeadlineExceeded
		}).AnyTimes()
	immediateFailGetter.EXPECT().GetEDS(gomock.Any(), gomock.Any()).
		Return(nil, errors.New("second getter fails immediately")).AnyTimes()
	successGetter.EXPECT().GetEDS(gomock.Any(), gomock.Any()).
		Return(nil, nil).AnyTimes()
	ctxGetter.EXPECT().GetEDS(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, _ *header.ExtendedHeader) (*rsmt2d.ExtendedDataSquare, error) {
			return nil, ctx.Err()
		}).AnyTimes()

	stuckGetter := mock.NewMockGetter(ctrl)
	stuckGetter.EXPECT().GetEDS(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, _ *header.ExtendedHeader) (*rsmt2d.ExtendedDataSquare, error) {
			<-ctx.Done()
			return nil, ctx.Err()
		}).AnyTimes()

	get := func(ctx context.Context, get shwap.Getter) (*rsmt2d.ExtendedDataSquare, error) {
		return get.GetEDS(ctx, nil)
	}

	t.Run("SuccessFirst", func(t *testing.T) {
		getters := []shwap.Getter{successGetter, timeoutGetter, immediateFailGetter}
		_, err := cascadeGetters(ctx, getters, get)
		assert.NoError(t, err)
	})

	t.Run("SuccessSecond", func(t *testing.T) {
		getters := []shwap.Getter{immediateFailGetter, successGetter}
		_, err := cascadeGetters(ctx, getters, get)
		assert.NoError(t, err)
	})

	t.Run("SuccessSecondAfterFirst", func(t *testing.T) {
		getters := []shwap.Getter{timeoutGetter, successGetter}
		_, err := cascadeGetters(ctx, getters, get)
		assert.NoError(t, err)
	})

	t.Run("SuccessAfterMultipleTimeouts", func(t *testing.T) {
		getters := []shwap.Getter{timeoutGetter, immediateFailGetter, timeoutGetter, timeoutGetter, successGetter}
		_, err := cascadeGetters(ctx, getters, get)
		assert.NoError(t, err)
	})

	t.Run("Error", func(t *testing.T) {
		getters := []shwap.Getter{immediateFailGetter, timeoutGetter, immediateFailGetter}
		_, err := cascadeGetters(ctx, getters, get)
		assert.Error(t, err)
		assert.Equal(t, strings.Count(err.Error(), "\n"), 2)
	})

	t.Run("Context Canceled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(ctx)
		cancel()
		getters := []shwap.Getter{ctxGetter, ctxGetter, ctxGetter}
		_, err := cascadeGetters(ctx, getters, get)
		assert.Error(t, err)
		assert.Equal(t, strings.Count(err.Error(), "\n"), 0)
	})

	t.Run("Single", func(t *testing.T) {
		getters := []shwap.Getter{successGetter}
		_, err := cascadeGetters(ctx, getters, get)
		assert.NoError(t, err)
	})

	t.Run("Stuck getter", func(t *testing.T) {
		getters := []shwap.Getter{stuckGetter, successGetter}
		_, err := cascadeGetters(ctx, getters, get)
		assert.NoError(t, err)
	})
}
