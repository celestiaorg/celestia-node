package state

import (
	"context"
	"fmt"
	lens "github.com/strangelove-ventures/lens/client"
)

// CoreAccessor implements Accessor over an RPC connection
// with a celestia-core node.
type CoreAccessor struct {
	client *lens.ChainClient
}

// NewCoreAccessor returns a new CoreAccessor with the
// given ChainClient.
func NewCoreAccessor(cc *lens.ChainClient) *CoreAccessor {
	return &CoreAccessor{
		client: cc,
	}
}

// CurrentBalance gets current balance of the node's account.
func (ca *CoreAccessor) CurrentBalance() (Balance, error) {
	coins, err := ca.client.QueryBalance(Token)
	if err != nil {
		return Balance{}, err
	}
	if len(coins) == 0 {
		// try to get self account info
		addr, err := ca.client.Address()
		if err != nil {
			log.Errorw("core-access: fetching own Address", "err", err)
		}
		return Balance{}, fmt.Errorf("no balance returned for own account: %v", addr)
	}
	// TODO @renaynay: is the first index always the Celestia-specific balance?
	return coins[0], nil
}


func (ca *CoreAccessor) AccountBalance(account Account) (Balance, error) {
	coins, err := ca.client.QueryBalanceWithAddress(account.GetAddress().String())
	if err != nil {
		return Balance{}, err
	}
	if len(coins) == 0 {
		return Balance{}, fmt.Errorf("no balance returned for account: %s", account.GetAddress().String())
	}
	return coins[0], nil
}

func (ca *CoreAccessor) SubmitTx(ctx context.Context, tx Msg) (*TxResponse, error) {
	return ca.client.SendMsg(ctx, tx)
}