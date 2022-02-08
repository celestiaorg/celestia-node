package state

import "context"

var Token = "celestia"

// Accessor represents the behaviors necessary for a user to
// query for state-related information and submit transactions/
// messages to the celestia network.
type Accessor interface {
	// CurrentBalance retrieves the Celestia coin balance
	// for the node's Account.
	CurrentBalance() (Balance, error)
	// AccountBalance retrieves the Celestia coin balance
	// for the given Account.
	AccountBalance(Account) (Balance, error)
	// SubmitTx submits the given transaction/message to the
	// Celestia network.
	// TODO @renaynay @wondertan: do we call this SubmitTx or SubmitMsg?
	SubmitTx(context.Context, Msg) (*TxResponse, error)
}
