// Package peers provides a peer manager that handles peer discovery and peer selection for the shrex getter.
//
// The peer manager is responsible for:
//   - Discovering peers
//   - Selecting peers for data retrieval
//   - Validating peers
//   - Blacklisting peers
//   - Garbage collecting peers
//
// The peer manager is not responsible for:
//   - Connecting to peers
//   - Disconnecting from peers
//   - Sending data to peers
//   - Receiving data from peers
//
// The peer manager is a mechanism to store peers from shrexsub, a mechanism that
// handles "peer discovery" and "peer selection" by relying on a shrexsub subscription
// and header subscriptions, such that it listens for new headers and
// new shares and uses this information to pool peers by shares.
//
// This gives the peer manager an ability to block peers that gossip invalid shares, but also access a list of peers
// that are known to have been gossiping valid shares.
// The peers are then returned on request using a round-robin algorithm to return a different peer each time.
// If no peers are found, the peer manager will rely on full nodes retrieved from discovery.
//
// The peer manager is only concerned with recent heights, thus it retrieves peers that
// were active since `initialHeight`.
// The peer manager will also garbage collect peers such that it blacklists peers that
// have been active since `initialHeight` but have been found to be invalid.
//
// The peer manager is passed to the shrex getter and is used at request time to
// select peers for a given data hash for data retrieval.
//
// # Usage
//
// The peer manager is created using [NewManager] constructor:
//
//	peerManager := peers.NewManager(headerSub, shrexSub, discovery, host, connGater, opts...)
//
// After creating the peer manager, it should be started to kick off listening and
// validation routines that enable peer selection and retrieval:
//
//	err := peerManager.Start(ctx)
//
// The peer manager can be stopped at any time to stop all peer discovery and validation routines:
//
//	err := peerManager.Stop(ctx)
//
// The peer manager can be used to select peers for a given datahash for shares retrieval:
//
//	peer, err := peerManager.Peer(ctx, hash)
package peers
