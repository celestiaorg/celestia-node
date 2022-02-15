package state

import "context"

// Accessor represents the behaviors necessary for a user to
// query for state-related information and submit transactions/
// messages to the Celestia network.
type Accessor interface {
	// Start starts the state Accessor.
	Start(context.Context) error
	// Stop stops the state Accessor.
	Stop(context.Context) error
	// Balance retrieves the Celestia coin balance
	// for the node's account/signer.
	Balance(ctx context.Context) (*Balance, error)
	// BalanceForAddress retrieves the Celestia coin balance
	// for the given address.
	BalanceForAddress(ctx context.Context, addr string) (*Balance, error)
	// SubmitTx submits the given transaction/message to the
	// Celestia network and blocks until a response is received.
	SubmitTx(ctx context.Context, tx Tx) (*TxResponse, error)
}
