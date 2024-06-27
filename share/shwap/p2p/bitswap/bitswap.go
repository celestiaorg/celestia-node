package bitswap

import (
	"context"
	"fmt"
	"time"

	"github.com/ipfs/boxo/bitswap/client"
	"github.com/ipfs/boxo/bitswap/network"
	"github.com/ipfs/boxo/bitswap/server"
	"github.com/ipfs/boxo/blockstore"
	delay "github.com/ipfs/go-ipfs-delay"
	routinghelpers "github.com/libp2p/go-libp2p-routing-helpers"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/protocol"
)

// Client constants
const (
	// simulateDontHaves emulates DONT_HAVE message from a peer after 5 second timeout.
	// This protects us from unresponsive/slow peers.
	// TODO(@Wondertan): PR to bitswap to make this timeout configurable
	//  Higher timeout increases the probability of successful reconstruction
	simulateDontHaves = true
	// providerSearchDelay defines the initial delay before Bitswap client starts aggressive
	// broadcasting of WANTs to all the peers. We offset this for longer than the default to minimize
	// unnecessary broadcasting as in most cases we already have peers connected with needed data on
	// a new request.
	providerSearchDelay = time.Second * 10
	// rebroadcastDelay is similar to the providerSearchDelay, but it targets DHT/ContentRouting
	// peer discovery and a gentle broadcast of a single random live WANT to all connected peers.
	// Considering no DHT usage and broadcasting configured by providerSearchDelay, we set
	// rebroadcastDelay to max value, effectively disabling it
	rebroadcastDelay = 1<<63 - 1
)

// Server constants
const (
	// providesEnabled dictates Bitswap Server not to provide content to DHT/ContentRouting as we don't use it
	providesEnabled = false
	// sendDontHaves prevents Bitswap Server from sending DONT_HAVEs while keeping peers on hold instead:
	//  * Clients simulate DONT_HAVEs after timeout anyway
	//  * Servers may not have data immediately and this gives an opportunity to subscribe
	//  * This is necessary for reconstruction. See https://github.com/celestiaorg/celestia-node/issues/732
	sendDontHaves = false
	// maxServerWantListsPerPeer defines the limit for maximum possible cached wants/requests per peer
	// in the Bitswap. Exceeding this limit will cause Bitswap server to drop requested wants leaving
	// client stuck for sometime.
	// Thus, we make the limit a bit generous, so we minimize the chances of this happening.
	// This is relevant until https://github.com/ipfs/boxo/pull/629#discussion_r1653362485 is fixed.
	maxServerWantListsPerPeer = 8096
	// targetMessageSize defines how much data Bitswap will aim to pack within a single message, before
	// splitting it up in multiple. Bitswap first looks up the size of the requested data across
	// multiple requests and only after reads up the data in portions one-by-one targeting the
	// targetMessageSize.
	//
	// Bigger number will speed transfers up if reading data from disk is fast. In our case, the
	// Bitswap's size lookup via [Blockstore] will already cause underlying cache to keep the data,
	// so reading up data is fast, and we can aim to pack as much as we can.
	targetMessageSize = 1 << 20 // 1MB
	// outstandingBytesPerPeer limits number of bytes queued for work for a peer across multiple requests.
	// We set it to be equal to targetMessageSize * N, so there can max N messages being prepared for
	// a peer at once.
	outstandingBytesPerPeer = targetMessageSize * 4
)

// NewNetwork constructs Bitswap network for Shwap protocol composition.
func NewNetwork(host host.Host, prefix protocol.ID) network.BitSwapNetwork {
	prefix = shwapProtocolID(prefix)
	net := network.NewFromIpfsHost(
		host,
		routinghelpers.Null{},
		network.Prefix(prefix),
		network.SupportedProtocols([]protocol.ID{protocolID}),
	)
	return net
}

// NewClient constructs a Bitswap client with parameters optimized for Shwap protocol composition.
// Meant to be used by Full and Light nodes.
func NewClient(ctx context.Context, net network.BitSwapNetwork, bstore blockstore.Blockstore) *client.Client {
	return client.New(
		ctx,
		net,
		bstore,
		client.SetSimulateDontHavesOnTimeout(simulateDontHaves),
		client.ProviderSearchDelay(providerSearchDelay),
		client.RebroadcastDelay(delay.Fixed(rebroadcastDelay)),
		// Prevents Has calls to Blockstore for metric that counts duplicates
		// Unnecessary for our use case, so we can save some disk lookups.
		client.WithoutDuplicatedBlockStats(),
	)
}

// NewServer construct a Bitswap server with parameters optimized for Shwap protocol composition.
// Meant to be used by Full nodes.
func NewServer(ctx context.Context, net network.BitSwapNetwork, bstore blockstore.Blockstore, opts ...server.Option) *server.Server {
	opts = append(
		opts,
		server.ProvideEnabled(providesEnabled),
		server.SetSendDontHaves(sendDontHaves),
		server.MaxQueuedWantlistEntriesPerPeer(maxServerWantListsPerPeer),
		server.WithTargetMessageSize(targetMessageSize),
		server.MaxOutstandingBytesPerPeer(outstandingBytesPerPeer),
	)

	return server.New(
		ctx,
		net,
		bstore,
		opts...,
	)
}

// TODO(@Wondertan): We have to use the protocol defined by Bitswap here
//
//	due to a little bug. Bitswap allows setting custom protocols, but
//	they have to be either one of the switch.
//	https://github.com/ipfs/boxo/blob/dfd4a53ba828a368cec8d61c3fe12969ac6aa94c/bitswap/network/ipfs_impl.go#L250-L266
var protocolID = network.ProtocolBitswap

func shwapProtocolID(network protocol.ID) protocol.ID {
	if network == "" {
		return ""
	}
	return protocol.ID(fmt.Sprintf("/%s/shwap", network))
}
