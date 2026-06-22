package client

import (
	"context"
	"fmt"
	"net/http"

	"github.com/filecoin-project/go-jsonrpc"

	"github.com/celestiaorg/celestia-node/libs/utils"
	blobapi "github.com/celestiaorg/celestia-node/nodebuilder/blob"
	blobstreamapi "github.com/celestiaorg/celestia-node/nodebuilder/blobstream"
	headerapi "github.com/celestiaorg/celestia-node/nodebuilder/header"
	shareapi "github.com/celestiaorg/celestia-node/nodebuilder/share"
)

// newJSONRPCClient opens a json-rpc client. It is a package-level variable so
// tests can simulate a mid-initialization failure.
var newJSONRPCClient = jsonrpc.NewClient

// Bridge node json-rpc connection config
type ReadConfig struct {
	// BridgeDAAddr is the address of the bridge node
	BridgeDAAddr string
	// DAAuthToken sets the value for Authorization http header
	DAAuthToken string
	// HTTPHeader contains custom headers that will be sent with each request
	HTTPHeader http.Header
	// EnableDATLS enables TLS for bridge node
	EnableDATLS bool
}

type ReadClient struct {
	Blob       blobapi.Module
	Header     headerapi.Module
	Share      shareapi.Module
	Blobstream blobstreamapi.Module

	closer func() error
}

func (cfg ReadConfig) Validate() error {
	if cfg.DAAuthToken != "" && cfg.HTTPHeader.Get("Authorization") != "" {
		return fmt.Errorf("authorization header already set, cannot use DAAuthToken as well")
	}
	_, err := utils.SanitizeAddr(cfg.BridgeDAAddr)
	return err
}

func NewReadClient(ctx context.Context, cfg ReadConfig) (*ReadClient, error) {
	err := cfg.Validate()
	if err != nil {
		return nil, err
	}

	if cfg.HTTPHeader == nil {
		cfg.HTTPHeader = http.Header{}
	}

	// Handle DAAuthToken logic
	if cfg.DAAuthToken != "" {
		cfg.HTTPHeader.Set("Authorization", "Bearer "+cfg.DAAuthToken)
		if !cfg.EnableDATLS {
			log.Warn("DAAuthToken is set but TLS is disabled, this is insecure")
		}
	}

	// Track every client opened so far and, unless construction succeeds, close
	// them on return. Otherwise a NewClient failure on a later client would leak
	// the connections already opened for the earlier ones.
	var closers []jsonrpc.ClientCloser
	success := false
	defer func() {
		if !success {
			for _, closer := range closers {
				closer()
			}
		}
	}()

	// Initialize share client
	shareAPI := shareapi.API{}
	shareCloser, err := newJSONRPCClient(
		ctx, cfg.BridgeDAAddr, "share", &shareAPI.Internal, cfg.HTTPHeader)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize share client: %w", err)
	}
	closers = append(closers, shareCloser)

	// Initialize blobstream client
	blobstreamAPI := blobstreamapi.API{}
	blobstreamCloser, err := newJSONRPCClient(
		ctx, cfg.BridgeDAAddr, "blobstream", &blobstreamAPI.Internal, cfg.HTTPHeader)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize blobstream client: %w", err)
	}
	closers = append(closers, blobstreamCloser)

	// Initialize header client
	headerAPI := headerapi.API{}
	headerCloser, err := newJSONRPCClient(
		ctx, cfg.BridgeDAAddr, "header", &headerAPI.Internal, cfg.HTTPHeader)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize header client: %w", err)
	}
	closers = append(closers, headerCloser)

	// Initialize blob read client
	blobAPI := blobapi.API{}
	blobCloser, err := newJSONRPCClient(
		ctx, cfg.BridgeDAAddr, "blob", &blobAPI.Internal, cfg.HTTPHeader)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize blob client: %w", err)
	}
	closers = append(closers, blobCloser)

	// pass prev func as value to avoid recursive call during unwrap
	closer := func() error {
		shareCloser()
		blobstreamCloser()
		headerCloser()
		blobCloser()
		return nil
	}
	success = true

	return &ReadClient{
		Share:      &shareAPI,
		Blobstream: &blobstreamAPI,
		Header:     &headerAPI,
		Blob:       &readOnlyBlobAPI{&blobAPI},
		closer:     closer,
	}, nil
}

// Close closes all open connections to Celestia consensus nodes and Bridge nodes.
func (c *ReadClient) Close() error {
	return c.closer()
}
