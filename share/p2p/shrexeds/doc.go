// This package defines a protocol that is used to request extended data from peers in the network.
//
// This protocol is request/response protocol that sends a request for extended data squares from peers in the network using data roots and receives a response with the extended data squares.
// The streams are established using the protocol ID: "{networkID}/shrex/eds/v0.0.1" where networkID is the network ID of the network. (e.g. "arabica")
//
// When a peer receives a request for extended data squares, it will respond with a CARv1 file containing the original data square
// and the client will take care of computing the extended data squares from the original data square on receipt.
//
// # Usage
//
// To use a shrexeds client to request extended data squares from a peer, you must first create a new `shrexeds.Client` instance by:
//
//	client, err := shrexeds.NewClient(params, host)
//
// where `params` is a `shrexeds.Parameters` instance and `host` is a `libp2p.Host` instance.
//
// To request extended data squares from a peer, you must first create a `Client.RequestEDS` instance by:
//
//	eds, err := client.RequestEDS(ctx, dataHash, peer)
//
// where `ctx` is a `context.Context` instance, `dataHash` is the data root of the extended data square and `peer` is the peer ID of the peer to request the extended data square from.
//
// To use a shrexeds server to respond to requests for extended data squares from peers, you must first create a new `shrexeds.Server` instance by:
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
