package cmd

import (
	"time"

	"github.com/libp2p/go-libp2p/core/metrics"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	ma2 "github.com/multiformats/go-multiaddr"
	"github.com/spf13/cobra"

	cmdnode "github.com/celestiaorg/celestia-node/cmd"
	"github.com/celestiaorg/celestia-node/nodebuilder/p2p"
)

type peerInfo struct {
	ID       string   `json:"id"`
	PeerAddr []string `json:"peer_addr"`
}

func init() {
	Cmd.AddCommand(infoCmd,
		peersCmd,
		peerInfoCmd,
		connectCmd,
		closePeerCmd,
		connectednessCmd,
		natStatusCmd,
		blockPeerCmd,
		unblockPeerCmd,
		blockedPeersCmd,
		protectCmd,
		unprotectCmd,
		protectedCmd,
		bandwidthStatsCmd,
		peerBandwidthCmd,
		bandwidthForProtocolCmd,
		pubsubPeersCmd,
		pubsubTopicsCmd,
		connectionInfoCmd,
		pingCmd,
	)
}

var Cmd = &cobra.Command{
	Use:               "p2p [command]",
	Short:             "Allows interaction with the P2P Module via JSON-RPC",
	Args:              cobra.NoArgs,
	PersistentPreRunE: cmdnode.InitClient,
}

var infoCmd = &cobra.Command{
	Use:   "info",
	Short: "Gets the node's peer info (peer id and multiaddresses)",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		client, err := cmdnode.ParseClientFromCtx(cmd.Context())
		if err != nil {
			return err
		}
		defer client.Close()

		info, err := client.P2P.Info(cmd.Context())

		formatter := func(data interface{}) interface{} {
			peerAdd := data.(peer.AddrInfo)
			ma := make([]string, len(info.Addrs))
			for i := range peerAdd.Addrs {
				ma[i] = peerAdd.Addrs[i].String()
			}

			return peerInfo{
				ID:       peerAdd.ID.String(),
				PeerAddr: ma,
			}
		}
		return cmdnode.PrintOutput(info, err, formatter)
	},
}

var peersCmd = &cobra.Command{
	Use:   "peers",
	Short: "Lists the peers we are connected to",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		client, err := cmdnode.ParseClientFromCtx(cmd.Context())
		if err != nil {
			return err
		}
		defer client.Close()

		result, err := client.P2P.Peers(cmd.Context())
		peers := make([]string, len(result))
		for i, peer := range result {
			peers[i] = peer.String()
		}

		formatter := func(data interface{}) interface{} {
			conPeers := data.([]string)
			return struct {
				Peers []string `json:"peers"`
			}{
				Peers: conPeers,
			}
		}
		return cmdnode.PrintOutput(peers, err, formatter)
	},
}

var peerInfoCmd = &cobra.Command{
	Use:   "peer-info [param]",
	Short: "Gets PeerInfo for a given peer",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := cmdnode.ParseClientFromCtx(cmd.Context())
		if err != nil {
			return err
		}
		defer client.Close()

		pid, err := peer.Decode(args[0])
		if err != nil {
			return err
		}
		info, err := client.P2P.PeerInfo(cmd.Context(), pid)
		formatter := func(data interface{}) interface{} {
			peerAdd := data.(peer.AddrInfo)
			ma := make([]string, len(info.Addrs))
			for i := range peerAdd.Addrs {
				ma[i] = peerAdd.Addrs[i].String()
			}

			return peerInfo{
				ID:       peerAdd.ID.String(),
				PeerAddr: ma,
			}
		}
		return cmdnode.PrintOutput(info, err, formatter)
	},
}

var connectCmd = &cobra.Command{
	Use:   "connect [peer.ID, address]",
	Short: "Establishes a connection with the given peer",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := cmdnode.ParseClientFromCtx(cmd.Context())
		if err != nil {
			return err
		}
		defer client.Close()

		pid, err := peer.Decode(args[0])
		if err != nil {
			return err
		}

		ma, err := ma2.NewMultiaddr(args[1])
		if err != nil {
			return err
		}

		peerInfo := peer.AddrInfo{
			ID:    pid,
			Addrs: []ma2.Multiaddr{ma},
		}

		err = client.P2P.Connect(cmd.Context(), peerInfo)
		if err != nil {
			return cmdnode.PrintOutput(nil, err, nil)
		}
		return connectednessCmd.RunE(cmd, args)
	},
}

var closePeerCmd = &cobra.Command{
	Use:   "close-peer [peer.ID]",
	Short: "Closes the connection with the given peer",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := cmdnode.ParseClientFromCtx(cmd.Context())
		if err != nil {
			return err
		}
		defer client.Close()

		pid, err := peer.Decode(args[0])
		if err != nil {
			return err
		}

		err = client.P2P.ClosePeer(cmd.Context(), pid)
		if err != nil {
			return cmdnode.PrintOutput(nil, err, nil)
		}
		return connectednessCmd.RunE(cmd, args)
	},
}

var connectednessCmd = &cobra.Command{
	Use:   "connectedness [peer.ID]",
	Short: "Checks the connection state between current and given peers",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := cmdnode.ParseClientFromCtx(cmd.Context())
		if err != nil {
			return err
		}
		defer client.Close()

		pid, err := peer.Decode(args[0])
		if err != nil {
			return err
		}

		con, err := client.P2P.Connectedness(cmd.Context(), pid)

		formatter := func(data interface{}) interface{} {
			conn := data.(network.Connectedness)
			return struct {
				ConnectionState string `json:"connection_state"`
			}{
				ConnectionState: conn.String(),
			}
		}
		return cmdnode.PrintOutput(con, err, formatter)
	},
}

var natStatusCmd = &cobra.Command{
	Use:   "nat-status",
	Short: "Gets the current NAT status",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		client, err := cmdnode.ParseClientFromCtx(cmd.Context())
		if err != nil {
			return err
		}
		defer client.Close()

		r, err := client.P2P.NATStatus(cmd.Context())

		formatter := func(data interface{}) interface{} {
			rr := data.(network.Reachability)
			return struct {
				Reachability string `json:"reachability"`
			}{
				Reachability: rr.String(),
			}
		}
		return cmdnode.PrintOutput(r, err, formatter)
	},
}

var blockPeerCmd = &cobra.Command{
	Use:   "block-peer [peer.ID]",
	Short: "Blocks the given peer",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := cmdnode.ParseClientFromCtx(cmd.Context())
		if err != nil {
			return err
		}
		defer client.Close()

		pid, err := peer.Decode(args[0])
		if err != nil {
			return err
		}

		err = client.P2P.BlockPeer(cmd.Context(), pid)

		formatter := func(data interface{}) interface{} {
			err, ok := data.(error)
			blocked := false
			if !ok {
				blocked = true
			}
			return struct {
				Blocked bool   `json:"blocked"`
				Peer    string `json:"peer"`
				Reason  error  `json:"reason,omitempty"`
			}{
				Blocked: blocked,
				Peer:    args[0],
				Reason:  err,
			}
		}
		return cmdnode.PrintOutput(err, nil, formatter)
	},
}

var unblockPeerCmd = &cobra.Command{
	Use:   "unblock-peer [peer.ID]",
	Short: "Unblocks the given peer",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := cmdnode.ParseClientFromCtx(cmd.Context())
		if err != nil {
			return err
		}
		defer client.Close()

		pid, err := peer.Decode(args[0])
		if err != nil {
			return err
		}

		err = client.P2P.UnblockPeer(cmd.Context(), pid)

		formatter := func(data interface{}) interface{} {
			err, ok := data.(error)
			unblocked := false
			if !ok {
				unblocked = true
			}

			return struct {
				Unblocked bool   `json:"unblocked"`
				Peer      string `json:"peer"`
				Reason    error  `json:"reason,omitempty"`
			}{
				Unblocked: unblocked,
				Peer:      args[0],
				Reason:    err,
			}
		}
		return cmdnode.PrintOutput(err, nil, formatter)
	},
}

var blockedPeersCmd = &cobra.Command{
	Use:   "blocked-peers",
	Short: "Lists the node's blocked peers",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		client, err := cmdnode.ParseClientFromCtx(cmd.Context())
		if err != nil {
			return err
		}
		defer client.Close()

		list, err := client.P2P.ListBlockedPeers(cmd.Context())

		pids := make([]string, len(list))
		for i, peer := range list {
			pids[i] = peer.String()
		}

		formatter := func(data interface{}) interface{} {
			peers := data.([]string)
			return struct {
				Peers []string `json:"peers"`
			}{
				Peers: peers,
			}
		}
		return cmdnode.PrintOutput(pids, err, formatter)
	},
}

var protectCmd = &cobra.Command{
	Use:   "protect [peer.ID, tag]",
	Short: "Protects the given peer from being pruned by the given tag",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := cmdnode.ParseClientFromCtx(cmd.Context())
		if err != nil {
			return err
		}
		defer client.Close()

		pid, err := peer.Decode(args[0])
		if err != nil {
			return err
		}

		err = client.P2P.Protect(cmd.Context(), pid, args[1])

		formatter := func(data interface{}) interface{} {
			err, ok := data.(error)
			protected := false
			if !ok {
				protected = true
			}
			return struct {
				Protected bool   `json:"protected"`
				Peer      string `json:"peer"`
				Reason    error  `json:"reason,omitempty"`
			}{
				Protected: protected,
				Peer:      args[0],
				Reason:    err,
			}
		}
		return cmdnode.PrintOutput(err, nil, formatter)
	},
}

var unprotectCmd = &cobra.Command{
	Use:   "unprotect [peer.ID, tag]",
	Short: "Removes protection from the given peer.",
	Long: "Removes a protection that may have been placed on a peer, under the specified tag." +
		"The return value indicates whether the peer continues to be protected after this call, by way of a different tag",
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := cmdnode.ParseClientFromCtx(cmd.Context())
		if err != nil {
			return err
		}
		defer client.Close()

		pid, err := peer.Decode(args[0])
		if err != nil {
			return err
		}

		_, err = client.P2P.Unprotect(cmd.Context(), pid, args[1])

		formatter := func(data interface{}) interface{} {
			err, ok := data.(error)
			unprotected := false
			if !ok {
				unprotected = true
			}
			return struct {
				Unprotected bool   `json:"unprotected"`
				Peer        string `json:"peer"`
				Reason      error  `json:"reason,omitempty"`
			}{
				Unprotected: unprotected,
				Peer:        args[0],
				Reason:      err,
			}
		}
		return cmdnode.PrintOutput(err, nil, formatter)
	},
}

var protectedCmd = &cobra.Command{
	Use:   "protected [peer.ID, tag]",
	Short: "Ensures that a given peer is protected under a specific tag",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := cmdnode.ParseClientFromCtx(cmd.Context())
		if err != nil {
			return err
		}
		defer client.Close()

		pid, err := peer.Decode(args[0])
		if err != nil {
			return err
		}

		result, err := client.P2P.IsProtected(cmd.Context(), pid, args[1])
		return cmdnode.PrintOutput(result, err, nil)
	},
}

type bandwidthStats struct {
	TotalIn  int64   `json:"total_in"`
	TotalOut int64   `json:"total_out"`
	RateIn   float64 `json:"rate_in"`
	RateOut  float64 `json:"rate_out"`
}

var bandwidthStatsCmd = &cobra.Command{
	Use:   "bandwidth-stats",
	Short: "Provides metrics for current peer.",
	Long: "Get stats struct with bandwidth metrics for all data sent/" +
		"received by the local peer, regardless of protocol or remote peer IDs",
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		client, err := cmdnode.ParseClientFromCtx(cmd.Context())
		if err != nil {
			return err
		}
		defer client.Close()

		result, err := client.P2P.BandwidthStats(cmd.Context())

		formatter := func(data interface{}) interface{} {
			stats := data.(metrics.Stats)
			return bandwidthStats{
				TotalIn:  stats.TotalIn,
				TotalOut: stats.TotalOut,
				RateIn:   stats.RateIn,
				RateOut:  stats.RateOut,
			}
		}
		return cmdnode.PrintOutput(result, err, formatter)
	},
}

var peerBandwidthCmd = &cobra.Command{
	Use:   "peer-bandwidth [peer.ID]",
	Short: "Gets stats struct with bandwidth metrics associated with the given peer.ID",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := cmdnode.ParseClientFromCtx(cmd.Context())
		if err != nil {
			return err
		}
		defer client.Close()

		pid, err := peer.Decode(args[0])
		if err != nil {
			return err
		}

		result, err := client.P2P.BandwidthForPeer(cmd.Context(), pid)

		formatter := func(data interface{}) interface{} {
			stats := data.(metrics.Stats)
			return bandwidthStats{
				TotalIn:  stats.TotalIn,
				TotalOut: stats.TotalOut,
				RateIn:   stats.RateIn,
				RateOut:  stats.RateOut,
			}
		}
		return cmdnode.PrintOutput(result, err, formatter)
	},
}

var bandwidthForProtocolCmd = &cobra.Command{
	Use:   "protocol-bandwidth [protocol.ID]",
	Short: "Gets stats struct with bandwidth metrics associated with the given protocol.ID",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := cmdnode.ParseClientFromCtx(cmd.Context())
		if err != nil {
			return err
		}
		defer client.Close()

		result, err := client.P2P.BandwidthForProtocol(cmd.Context(), protocol.ID(args[0]))

		formatter := func(data interface{}) interface{} {
			stats := data.(metrics.Stats)
			return bandwidthStats{
				TotalIn:  stats.TotalIn,
				TotalOut: stats.TotalOut,
				RateIn:   stats.RateIn,
				RateOut:  stats.RateOut,
			}
		}
		return cmdnode.PrintOutput(result, err, formatter)
	},
}

var pubsubPeersCmd = &cobra.Command{
	Use:   "pubsub-peers [topic]",
	Short: "Lists the peers we are connected to in the given topic",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := cmdnode.ParseClientFromCtx(cmd.Context())
		if err != nil {
			return err
		}
		defer client.Close()

		result, err := client.P2P.PubSubPeers(cmd.Context(), args[0])
		peers := make([]string, len(result))

		for i, peer := range result {
			peers[i] = peer.String()
		}

		formatter := func(data interface{}) interface{} {
			conPeers := data.([]string)
			return struct {
				Peers []string `json:"peers"`
			}{
				Peers: conPeers,
			}
		}
		return cmdnode.PrintOutput(peers, err, formatter)
	},
}

var pubsubTopicsCmd = &cobra.Command{
	Use:   "pubsub-topics ",
	Short: "Lists pubsub(GossipSub) topics the node participates in",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		client, err := cmdnode.ParseClientFromCtx(cmd.Context())
		if err != nil {
			return err
		}
		defer client.Close()

		topics, err := client.P2P.PubSubTopics(cmd.Context())
		formatter := func(data interface{}) interface{} {
			conPeers := data.([]string)
			return struct {
				Topics []string `json:"topics"`
			}{
				Topics: conPeers,
			}
		}
		return cmdnode.PrintOutput(topics, err, formatter)
	},
}

var connectionInfoCmd = &cobra.Command{
	Use:   "connection-state [peerID]",
	Short: "Gets connection info for a given peer ID",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := cmdnode.ParseClientFromCtx(cmd.Context())
		if err != nil {
			return err
		}
		defer client.Close()

		pid, err := peer.Decode(args[0])
		if err != nil {
			return err
		}

		infos, err := client.P2P.ConnectionState(cmd.Context(), pid)
		return cmdnode.PrintOutput(infos, err, func(i interface{}) interface{} {
			type state struct {
				Info       network.ConnectionState
				NumStreams int
				Direction  string
				Opened     string
				Limited    bool
			}

			states := i.([]p2p.ConnectionState)
			infos := make([]state, len(states))
			for i, s := range states {
				infos[i] = state{
					Info:       s.Info,
					NumStreams: s.NumStreams,
					Direction:  s.Direction.String(),
					Opened:     s.Opened.Format(time.DateTime),
					Limited:    s.Limited,
				}
			}

			if len(infos) == 1 {
				return infos[0]
			}
			return infos
		})
	},
}

var pingCmd = &cobra.Command{
	Use:   "ping [peerID]",
	Short: "Pings given peer and tell how much time that took or errors",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := cmdnode.ParseClientFromCtx(cmd.Context())
		if err != nil {
			return err
		}
		defer client.Close()

		pid, err := peer.Decode(args[0])
		if err != nil {
			return err
		}

		pingDuration, err := client.P2P.Ping(cmd.Context(), pid)
		return cmdnode.PrintOutput(pingDuration, err, func(i interface{}) interface{} {
			return i.(time.Duration).String()
		})
	},
}
