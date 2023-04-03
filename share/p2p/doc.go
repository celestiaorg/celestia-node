// Package p2p provides p2p functionality that powers the share exchange protocols used by celestia-node.
// The available protocols are:
//
//   - shrexsub : a floodsub-based protocol that is used to broadcast shares to peers. This protocol is pubsub protocol
//     that broadcasts and listens for shares over a pubsub topic.
//
//   - shrexnd: a request/response protocol that is used to request shares from peers.
//
//   - shrexeds: a request/response protocol that is used to request extended data square shares from peers.
//     This protocol exchanges the original data square in between the client and server as a CARv1 file,
//     and it's up to the receiver to compute the extended data square.
//
// This package also defines a peer manager that is used to manage network peers that can be used to exchange
// shares. The peer manager is primarily responsible for providing peers to request shares from,
// and is primarily used by `getters.ShrexGetter` in share/getters/shrex.go.
//
// Find out more about each protocol in their respective sub-packages.
package p2p
