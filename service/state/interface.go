package state

import (
	"context"

	"google.golang.org/grpc"

	apptypes "github.com/celestiaorg/celestia-app/x/payment/types"
)

// Accessor represents the behaviors necessary for a user to
// query for state-related information and submit transactions/
// messages to the Celestia network.
type Accessor interface {
	// Start starts the state Accessor.
	Start(context.Context) error
	// Stop stops the state Accessor.
	Stop(context.Context) error

	// KeyringSigner returns the KeyringSigner used by the Accessor.
	KeyringSigner() *apptypes.KeyringSigner
	// Conn returns a gRPC connection to node that serves state-related
	// requests.
	Conn() (*grpc.ClientConn, error)

	// Balance retrieves the Celestia coin balance
	// for the node's account/signer.
	Balance(ctx context.Context) (*Balance, error)
	// BalanceForAddress retrieves the Celestia coin balance
	// for the given types.AccAddress.
	BalanceForAddress(ctx context.Context, addr Address) (*Balance, error)
	// SubmitTx submits the given transaction/message to the
	// Celestia network and blocks until the tx is included in
	// a block.
	SubmitTx(ctx context.Context, tx Tx) (*TxResponse, error)
}
