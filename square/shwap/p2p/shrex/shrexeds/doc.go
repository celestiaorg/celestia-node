// This package defines a protocol that is used to request
// extended data squares from peers in the network.
//
// This protocol is a request/response protocol that allows for sending requests for extended data squares by data root
// to the peers in the network and receiving a response containing the original data square(s), which is used
// to recompute the extended data square.
//
// The streams are established using the protocol ID:
//
//   - "{networkID}/shrex/eds/v0.0.1" where networkID is the network ID of the network. (e.g. "arabica")
//
// When a peer receives a request for extended data squares, it will read
// the original data square from the EDS store by retrieving the underlying
// CARv1 file containing the full extended data square, but will limit reading
// to the original data square shares only.
// The client on the other hand will take care of computing the extended data squares from
// the original data square on receipt.
//
// # Usage
//
// To use a shrexeds client to request extended data squares from a peer, you must
// first create a new `shrexeds.Client` instance by:
//
//	client, err := shrexeds.NewClient(params, host)
//
// where `params` is a `shrexeds.Parameters` instance and `host` is a `libp2p.Host` instance.
//
// To request extended data squares from a peer, you must first create a `Client.RequestEDS` instance by:
//
//	eds, err := client.RequestEDS(ctx, dataHash, peer)
//
// where:
//   - `ctx` is a `context.Context` instance,
//   - `dataHash` is the data root of the extended data square and
//   - `peer` is the peer ID of the peer to request the extended data square from.
//
// To use a shrexeds server to respond to requests for extended data squares from peers
// you must first create a new `shrexeds.Server` instance by:
//
//	server, err := shrexeds.NewServer(params, host, store)
//
// where `params` is a [Parameters] instance, `host` is a libp2p.Host instance and `store` is a [eds.Store] instance.
//
// To start the server, you must call `Start` on the server:
//
//	err := server.Start(ctx)
//
// To stop the server, you must call `Stop` on the server:
//
//	err := server.Stop(ctx)
package shrexeds
