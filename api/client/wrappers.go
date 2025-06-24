package client

import (
	"context"
	"errors"

	libhead "github.com/celestiaorg/go-header"

	"github.com/celestiaorg/celestia-node/blob"
	"github.com/celestiaorg/celestia-node/header"
	blobapi "github.com/celestiaorg/celestia-node/nodebuilder/blob"
	headerapi "github.com/celestiaorg/celestia-node/nodebuilder/header"
)

var _ blobapi.Module = (*blobSubmitClient)(nil)

var ErrReadOnlyMode = errors.New("submit is disabled in read only client")

type readOnlyBlobAPI struct {
	blobapi.Module
}

type blobSubmitClient struct {
	blobapi.Module
	submitter *blob.Service
}

func (api *readOnlyBlobAPI) Submit(context.Context, []*blob.Blob, *blob.SubmitOptions) (uint64, error) {
	return 0, ErrReadOnlyMode
}

func (api *blobSubmitClient) Submit(ctx context.Context,
	blobs []*blob.Blob, options *blob.SubmitOptions,
) (uint64, error) {
	if api.submitter == nil {
		return 0, errors.New("key needs to be set before blob.Submit can be used")
	}
	// TODO: this is a hack to allow nil options, because it's not exported by the blob package
	if options == nil {
		options = &blob.SubmitOptions{}
	}
	return api.submitter.Submit(ctx, blobs, options)
}

type trustedHeadGetter struct {
	remote headerapi.Module
}

func (t trustedHeadGetter) Head(
	ctx context.Context,
	_ ...libhead.HeadOption[*header.ExtendedHeader],
) (*header.ExtendedHeader, error) {
	return t.remote.NetworkHead(ctx)
}
