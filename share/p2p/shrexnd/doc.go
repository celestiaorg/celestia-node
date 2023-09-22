// This package defines a protocol that is used to request namespaced data from peers in the network.
//
// This protocol is a request/response protocol that sends a request for specific data that
// lives in a specific namespace ID and receives a response with the data.
//
// The streams are established using the protocol ID:
//
//   - "{networkID}/shrex/nd/0.0.1" where networkID is the network ID of the network. (e.g. "arabica")
//
// The protocol uses protobuf to serialize and deserialize messages.
//
// # Usage
//
// To use a shrexnd client to request data from a peer, you must first create a new `shrexnd.Client` instance by:
//
// 1. Create a new client using `NewClient` and pass in the parameters of the protocol and the host:
//
//	client, err := shrexnd.NewClient(params, host)
//
// 2. Request data from a peer by calling [Client.RequestND] on the client and
// pass in the context, the data root, the namespace ID and the peer ID:
//
//	data, err := client.RequestND(ctx, dataRoot, peerID, namespaceID)
//
// where data is of type [share.NamespacedShares]
//
// To use a shrexnd server to respond to requests from peers, you must first create a new `shrexnd.Server` instance by:
//
// 1. Create a new server using `NewServer` and pass in the parameters of
// the protocol, the host, the store and store share getter:
//
//	server, err := shrexnd.NewServer(params, host, store, storeShareGetter)
//
// where store is of type [share.Store] and storeShareGetter is of type [share.Getter]
//
// 2. Start the server by calling `Start` on the server:
//
//	err := server.Start(ctx)
//
// 3. Stop the server by calling `Stop` on the server:
//
//	err := server.Stop(ctx)
package shrexnd
