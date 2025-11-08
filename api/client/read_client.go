package client

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/filecoin-project/go-jsonrpc"

	"github.com/celestiaorg/celestia-node/libs/utils"
	blobapi "github.com/celestiaorg/celestia-node/nodebuilder/blob"
	blobstreamapi "github.com/celestiaorg/celestia-node/nodebuilder/blobstream"
	fraudapi "github.com/celestiaorg/celestia-node/nodebuilder/fraud"
	headerapi "github.com/celestiaorg/celestia-node/nodebuilder/header"
	shareapi "github.com/celestiaorg/celestia-node/nodebuilder/share"
)

// MultiReadClient manages multiple bridge DA connections with failover support
type MultiReadClient struct {
	clients []*ReadClient
	configs []ReadConfig
}

// NewMultiReadClient creates a new multi-endpoint read client
func NewMultiReadClient(ctx context.Context, cfg ReadConfig) (*MultiReadClient, error) {
	// Collect all configurations (primary + additional)
	configs := []ReadConfig{cfg}
	for _, addr := range cfg.AdditionalBridgeDAAddrs {
		additionalCfg := cfg
		additionalCfg.BridgeDAAddr = addr
		additionalCfg.AdditionalBridgeDAAddrs = nil // Avoid infinite recursion
		configs = append(configs, additionalCfg)
	}

	// Create clients for all configurations
	clients := make([]*ReadClient, 0, len(configs))
	for _, config := range configs {
		client, err := NewReadClient(ctx, config)
		if err != nil {
			// Close any already created clients on error
			for _, existingClient := range clients {
				existingClient.Close()
			}
			return nil, fmt.Errorf("failed to create read client for %s: %w", config.BridgeDAAddr, err)
		}
		clients = append(clients, client)
	}

	return &MultiReadClient{
		clients: clients,
		configs: configs,
	}, nil
}

// GetClient returns the first available client
func (m *MultiReadClient) GetClient() *ReadClient {
	if len(m.clients) > 0 {
		return m.clients[0]
	}
	return nil
}

// GetAllClients returns all clients for advanced usage
func (m *MultiReadClient) GetAllClients() []*ReadClient {
	return m.clients
}

// Close closes all read clients
func (m *MultiReadClient) Close() error {
	var errs []error
	for i, client := range m.clients {
		if err := client.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close client %d (%s): %w", i, m.configs[i].BridgeDAAddr, err))
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

// Bridge node json-rpc connection config
type ReadConfig struct {
	// BridgeDAAddr is the address of the bridge node
	BridgeDAAddr string
	// AdditionalBridgeDAAddrs is a list of additional bridge node addresses for failover
	AdditionalBridgeDAAddrs []string
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
	Fraud      fraudapi.Module
	Blobstream blobstreamapi.Module

	closer func() error
}

func (cfg ReadConfig) Validate() error {
	if cfg.DAAuthToken != "" && cfg.HTTPHeader.Get("Authorization") != "" {
		return fmt.Errorf("authorization header already set, cannot use DAAuthToken as well")
	}
	_, err := utils.SanitizeAddr(cfg.BridgeDAAddr)
	if err != nil {
		return err
	}

	// Validate additional bridge addresses
	for idx, addr := range cfg.AdditionalBridgeDAAddrs {
		_, err := utils.SanitizeAddr(addr)
		if err != nil {
			return fmt.Errorf("invalid additional bridge address at index %d: %w", idx, err)
		}
	}

	return nil
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

	// Initialize share client
	shareAPI := shareapi.API{}
	shareCloser, err := jsonrpc.NewClient(
		ctx, cfg.BridgeDAAddr, "share", &shareAPI.Internal, cfg.HTTPHeader)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize share client: %w", err)
	}

	// Initialize blobstream client
	blobstreamAPI := blobstreamapi.API{}
	blobstreamCloser, err := jsonrpc.NewClient(
		ctx, cfg.BridgeDAAddr, "blobstream", &blobstreamAPI.Internal, cfg.HTTPHeader)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize blobstream client: %w", err)
	}

	// Initialize header client
	headerAPI := headerapi.API{}
	headerCloser, err := jsonrpc.NewClient(
		ctx, cfg.BridgeDAAddr, "header", &headerAPI.Internal, cfg.HTTPHeader)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize header client: %w", err)
	}

	fraudAPI := fraudapi.API{}
	fraudCloser, err := jsonrpc.NewClient(
		ctx, cfg.BridgeDAAddr, "fraud", &fraudAPI.Internal, cfg.HTTPHeader)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize fraud client: %w", err)
	}

	// Initialize blob read client
	blobAPI := blobapi.API{}
	blobCloser, err := jsonrpc.NewClient(
		ctx, cfg.BridgeDAAddr, "blob", &blobAPI.Internal, cfg.HTTPHeader)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize blob client: %w", err)
	}

	// pass prev func as value to avoid recursive call during unwrap
	closer := func() error {
		shareCloser()
		blobstreamCloser()
		headerCloser()
		fraudCloser()
		blobCloser()
		return nil
	}

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
