package bitswap

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ipfs/boxo/bitswap/client"
	"github.com/ipfs/boxo/bitswap/network"
	"github.com/ipfs/boxo/bitswap/network/bsnet"
	"github.com/ipfs/boxo/bitswap/server"
	"github.com/ipfs/boxo/blockstore"
	blocks "github.com/ipfs/go-block-format"
	delay "github.com/ipfs/go-ipfs-delay"
	routinghelpers "github.com/libp2p/go-libp2p-routing-helpers"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/protocol"
)

// THESE VALUES HAVE TO BE REVISED ON EVERY SINGLE MAX BLOCK SIZE BUMP
// CURRENT BLOCK SIZE TARGET: 8MB (256 EDS)

// Client constants
const (
	// simulateDontHaves emulates DONT_HAVE message from a peer after dynamically estimated timeout.
	// Simulating protects from malicious peers and ensure Bitswap tries new peers if originally
	// selected one is slow or not responding.
	// Dynamic timeout, in our case, can be treated as optimization allowing to move on a next peer faster.
	// See simulateDontHaveConfig for more details.
	simulateDontHaves = true
	// broadcastDelay defines the initial delay before Bitswap client starts aggressive
	// broadcasting of live WANTs to all the peers. We offset this for longer than the default to minimize
	// unnecessary broadcasting as in most cases we already have peers connected with needed data on
	// a new request.
	broadcastDelay = time.Second * 10
	// provSearchDelay is similar to the broadcastDelay, but it targets DHT/ContentRouting
	// peer discovery and a gentle broadcast of a single random live WANT to all connected peers.
	// Considering no DHT usage and broadcasting configured by broadcastDelay, we set
	// provSearchDelay to max value, effectively disabling it
	provSearchDelay = 1<<63 - 1
)

// Server constants
const (
	// sendDontHaves dictates Bitswap Server to send DONT_HAVE messages to peers when they requested block is not
	// available locally. This is useful for clients to quickly move on to another peer.
	// This breaks reconstruction, unless we make reconstruction case detectable on the Server side blocking Bitswap
	// from serving DONT_HAVE messages in Blockstore, which would be the goal.
	// TODO(@Wondertan): enable once Blockstore handles recent blocks
	sendDontHaves = false
	// maxServerWantListsPerPeer defines the limit for maximum possible cached wants/requests per peer
	// in the Bitswap. Exceeding this limit will cause Bitswap server to drop requested WANTs leaving
	// client stuck for some time.
	// This is relevant until https://github.com/ipfs/boxo/pull/629#discussion_r1653362485 is fixed.
	// 1024 is 64 sampling requests of size 16 and 8 EDS requests with 8mb blocks
	maxServerWantListsPerPeer = 4096
	// targetResponseSize defines soft-limit of how much data server packs into a response.
	// More data means more compute and time spend while serving a single peer under load, and thus increasing serving
	// latency for other peers.
	// We set it to 65KiB, which fits a Row of 8MB block with additional metadata.
	targetResponseSize = 65 << 10
	// responseWorkersCount is the number of workers packing responses on the server.
	// More workers mean more parallelism and faster responses, but also more memory usage.
	// Default is 8.
	responseWorkersCount = 32
	// maxWorkPerPeer sets maximum concurrent processing work allowed for a peer.
	// We set it to be equal to targetResponseSize * responseWorkersCount, so a single peer
	// can utilize all workers. Note that when there are more peers, prioritization still ensures
	// even spread across all peers.
	maxWorkPerPeer = targetResponseSize * responseWorkersCount
	// replaceHasWithBlockMaxSize configures Bitswap to use Has method instead of GetSize to check existence
	// of a CID in Blockstore.
	replaceHasWithBlockMaxSize = 0
)

// simulateDontHaveConfig contains the configuration for Bitswap's DONT_HAVE simulation.
// This relies on latency to determine when to simulate a DONT_HAVE from a peer.
var simulateDontHaveConfig = &client.DontHaveTimeoutConfig{
	// MaxTimeout is the limit cutoff time for the dynamic timeout estimation.
	MaxTimeout: 30 * time.Second,
	// MinTimeout is the minimum timeout for the dynamic timeout estimation.
	MinTimeout: 1 * time.Second,
	// MessageLatencyMultiplier is the multiplier for the message latency to account for errors
	// THIS IS THE MOST IMPORTANT KNOB and is tricky to get right due to high variance in latency
	// and particularly request processing time.
	MessageLatencyMultiplier: 4,
	// MessageLatencyAlpha is the alpha value for the exponential moving average of message latency.
	// It's a 0-1 value. More prefers recent latencies, less prefers older measurements.
	MessageLatencyAlpha: 0.2,

	// Below are less important knobs and target initial timeouts for when there is no latency data yet
	//
	// DontHaveTimeout is default timeout used until ping latency is measured
	DontHaveTimeout: 10 * time.Second,
	// time estimate for how long it takes to process a WANT/Block message
	// used for ping timeout calculation
	MaxExpectedWantProcessTime: 7 * time.Second,
	// multiplier to account for errors with ping latency estimates
	PingLatencyMultiplier: 3,
}

// NewNetwork constructs Bitswap network for Shwap protocol composition.
func NewNetwork(host host.Host, prefix protocol.ID) network.BitSwapNetwork {
	prefix = shwapProtocolID(prefix)
	net := bsnet.NewFromIpfsHost(
		host,
		bsnet.Prefix(prefix),
		bsnet.SupportedProtocols([]protocol.ID{protocolID}),
	)
	return net
}

// NewClient constructs a Bitswap client with parameters optimized for Shwap protocol composition.
// Meant to be used by Full and Light nodes.
func NewClient(
	ctx context.Context,
	net network.BitSwapNetwork,
	bstore blockstore.Blockstore,
) *client.Client {
	opts := []client.Option{
		client.SetSimulateDontHavesOnTimeout(simulateDontHaves),
		client.WithDontHaveTimeoutConfig(simulateDontHaveConfig),
		// Prevents Has calls to Blockstore for metric that counts duplicates
		// Unnecessary for our use case, so we can save some disk lookups.
		client.WithoutDuplicatedBlockStats(),

		// These two options have mixed up named. One should be another and vice versa.
		client.ProviderSearchDelay(broadcastDelay),
		client.RebroadcastDelay(delay.Fixed(provSearchDelay)),
	}
	return client.New(
		ctx,
		net,
		routinghelpers.Null{},
		bstore,
		opts...,
	)
}

// NewServer construct a Bitswap server with parameters optimized for Shwap protocol composition.
// Meant to be used by Full nodes.
func NewServer(
	ctx context.Context,
	net network.BitSwapNetwork,
	bstore blockstore.Blockstore,
) *server.Server {
	opts := []server.Option{
		server.SetSendDontHaves(sendDontHaves),
		server.MaxQueuedWantlistEntriesPerPeer(maxServerWantListsPerPeer),
		server.WithTargetMessageSize(targetResponseSize),
		server.MaxOutstandingBytesPerPeer(maxWorkPerPeer),
		server.WithWantHaveReplaceSize(replaceHasWithBlockMaxSize),
		server.TaskWorkerCount(responseWorkersCount),
	}
	return server.New(ctx, net, bstore, opts...)
}

type Bitswap struct {
	*client.Client
	*server.Server
}

func New(
	ctx context.Context,
	net network.BitSwapNetwork,
	bstore blockstore.Blockstore,
) *Bitswap {
	return &Bitswap{
		Client: NewClient(ctx, net, bstore),
		Server: NewServer(ctx, net, bstore),
	}
}

func (bs *Bitswap) NotifyNewBlocks(ctx context.Context, blks ...blocks.Block) error {
	return errors.Join(
		bs.Client.NotifyNewBlocks(ctx, blks...),
		bs.Server.NotifyNewBlocks(ctx, blks...),
	)
}

func (bs *Bitswap) Close() error {
	bs.Server.Close()
	return bs.Client.Close()
}

// TODO(@Wondertan): We have to use the protocol defined by Bitswap here
//
//	due to a little bug. Bitswap allows setting custom protocols, but
//	they have to be either one of the switch.
//	https://github.com/ipfs/boxo/blob/dfd4a53ba828a368cec8d61c3fe12969ac6aa94c/bitswap/network/ipfs_impl.go#L250-L266
var protocolID = bsnet.ProtocolBitswap

func shwapProtocolID(network protocol.ID) protocol.ID {
	if network == "" {
		return ""
	}
	return protocol.ID(fmt.Sprintf("%s/shwap", network))
}
