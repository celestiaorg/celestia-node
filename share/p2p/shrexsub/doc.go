// This package defines a protocol that is used to broadcast shares to peers over a pubsub network.
//
// This protocol runs on a rudimentary floodsub network is primarily a pubsub protocol
// that broadcasts and listens for shares over a pubsub topic.
//
// The pubsub topic used by this protocol is:
//
//	"{networkID}/eds-sub/v0.1.0"
//
// where networkID is the network ID of the celestia-node that is running the protocol. (e.g. "arabica")
//
// # Usage
//
// To use this protocol, you must first create a new `shrexsub.PubSub` instance by:
//
//	pubsub, err := shrexsub.NewPubSub(ctx, host, networkID)
//
// where host is the libp2p host that is running the protocol, and networkID is the network ID of the celestia-node
// that is running the protocol.
//
// After this, you can start the pubsub protocol by:
//
//	err := pubsub.Start(ctx)
//
// Once you have started the `shrexsub.PubSub` instance, you can broadcast a share by:
//
//	err := pubsub.Broadcast(ctx, notification)
//
// where `notification` is of type [shrexsub.Notification].
//
// and `DataHash` is the hash of the share that you want to broadcast, and `Height` is the height of the share.
//
// You can also subscribe to the pubsub topic by:
//
//	sub, err := pubsub.Subscribe(ctx)
//
// and then receive notifications by:
//
//	  for {
//	  	select {
//	  	case <-ctx.Done():
//				sub.Cancel()
//	  		return
//	  	case notification, err := <-sub.Next():
//	  		// handle notification or err
//	  	}
//	  }
//
// You can also manipulate the received pubsub messages by using the [PubSub.AddValidator] method:
//
//	pubsub.AddValidator(validator ValidatorFn)
//
// where `validator` is of type [shrexsub.ValidatorFn] and `Notification` is the same as above.
//
// You can also stop the pubsub protocol by:
//
//	err := pubsub.Stop(ctx)
package shrexsub
