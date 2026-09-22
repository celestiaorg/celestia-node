package canary

import (
	"context"
	"errors"
	"io"
	"sync"

	"github.com/libp2p/go-libp2p/core/peer"

	libhead "github.com/celestiaorg/go-header"

	rpcclient "github.com/celestiaorg/celestia-node/api/rpc/client"
	"github.com/celestiaorg/celestia-node/das"
	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/nodebuilder/tests/tastora/canary/model"
)

// NativeSession is the Docker session as the engine uses it. Admin tokens exist
// only inside P2PInfo and P2PPeers; RPC returns a read-only client.
type NativeSession interface {
	Start(context.Context) (model.ProcessInfo, error)
	Stop(context.Context) (model.ProcessInfo, error)
	Inspect(context.Context) (model.ProcessInfo, error)
	RPC(context.Context) (*rpcclient.Client, error)
	P2PInfo(context.Context) (peer.AddrInfo, error)
	P2PPeers(context.Context) ([]peer.ID, error)
	Logs(context.Context) (io.ReadCloser, error)
	Store() model.StoreIdentity
	Cleanup(context.Context) error
}
type nativeSession struct{ NativeSession }

func AdaptSession(s NativeSession) Session {
	if s == nil {
		return nil
	}
	return nativeSession{s}
}

func (s nativeSession) RPC(ctx context.Context) (Reader, error) {
	c, err := s.NativeSession.RPC(ctx)
	if err != nil {
		return nil, err
	}
	if c == nil {
		return nil, errors.New("native RPC unavailable")
	}
	return &nativeReader{client: c, metadata: s.NativeSession}, nil
}

// nativeReader wraps the client instead of embedding it: read permission also
// allows share retrieval, which the engine must not use.
type nativeReader struct {
	client   *rpcclient.Client
	metadata NativeSession
	close    sync.Once
}

func (r *nativeReader) Info(ctx context.Context) (peer.AddrInfo, error) {
	return r.metadata.P2PInfo(ctx)
}

func (r *nativeReader) Peers(ctx context.Context) ([]peer.ID, error) { return r.metadata.P2PPeers(ctx) }

func (r *nativeReader) LocalHead(ctx context.Context) (*header.ExtendedHeader, error) {
	return r.client.Header.LocalHead(ctx)
}

func (r *nativeReader) NetworkHead(ctx context.Context) (*header.ExtendedHeader, error) {
	return r.client.Header.NetworkHead(ctx)
}

func (r *nativeReader) Tail(ctx context.Context) (*header.ExtendedHeader, error) {
	return r.client.Header.Tail(ctx)
}

func (r *nativeReader) GetByHeight(ctx context.Context, h uint64) (*header.ExtendedHeader, error) {
	return r.client.Header.GetByHeight(ctx, h)
}

func (r *nativeReader) GetByHash(ctx context.Context, h libhead.Hash) (*header.ExtendedHeader, error) {
	return r.client.Header.GetByHash(ctx, h)
}

func (r *nativeReader) GetRangeByHeight(
	ctx context.Context,
	from *header.ExtendedHeader,
	to uint64,
) ([]*header.ExtendedHeader, error) {
	return r.client.Header.GetRangeByHeight(ctx, from, to)
}

func (r *nativeReader) SamplingStats(ctx context.Context) (das.SamplingStats, error) {
	return r.client.DAS.SamplingStats(ctx)
}
func (r *nativeReader) Close() error { r.close.Do(r.client.Close); return nil }
