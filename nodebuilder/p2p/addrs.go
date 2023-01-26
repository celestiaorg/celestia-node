package p2p

import (
	"fmt"

	p2pconfig "github.com/libp2p/go-libp2p/config"
	hst "github.com/libp2p/go-libp2p/core/host"
	ma "github.com/multiformats/go-multiaddr"
)

// Listen returns invoke function that starts listening for inbound connections with libp2p.Host.
func Listen(listen []string) func(h hst.Host) (err error) {
	return func(h hst.Host) (err error) {
		maListen := make([]ma.Multiaddr, len(listen))
		for i, addr := range listen {
			maListen[i], err = ma.NewMultiaddr(addr)
			if err != nil {
				return fmt.Errorf("failure to parse config.P2P.ListenAddresses: %s", err)
			}
		}
		return h.Network().Listen(maListen...)
	}
}

// addrsFactory returns a constructor for AddrsFactory.
func addrsFactory(announce []string, noAnnounce []string) func() (_ p2pconfig.AddrsFactory, err error) {
	return func() (_ p2pconfig.AddrsFactory, err error) {
		// Convert maAnnounce strings to Multiaddresses
		maAnnounce := make([]ma.Multiaddr, len(announce))
		for i, addr := range announce {
			maAnnounce[i], err = ma.NewMultiaddr(addr)
			if err != nil {
				return nil, fmt.Errorf("failure to parse config.P2P.AnnounceAddresses: %s", err)
			}
		}

		// TODO(@Wondertan): Support filtering with network masks for noAnnounce, e.g. 255.255.255.0
		// Collect all addresses that should not be announced
		maNoAnnounce := make(map[string]bool, len(noAnnounce))
		for _, addr := range noAnnounce {
			maddr, err := ma.NewMultiaddr(addr)
			if err != nil {
				return nil, fmt.Errorf("failure to parse config.P2P.NoAnnounceAddresses: %s", err)
			}
			maNoAnnounce[string(maddr.Bytes())] = true
		}

		return func(maListen []ma.Multiaddr) []ma.Multiaddr {
			// copy maAnnounce to out
			out := make([]ma.Multiaddr, 0, len(maAnnounce)+len(maListen))
			out = append(out, maAnnounce...)

			// filter out unneeded
			for _, maddr := range maListen {
				ok := maNoAnnounce[string(maddr.Bytes())]
				if !ok {
					out = append(out, maddr)
				}
			}
			return out
		}, nil
	}
}
