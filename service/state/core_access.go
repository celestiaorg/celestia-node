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
	acc, err := ca.client.AccountFromKeyOrAddress(ca.client.Key())
	if err != nil {
		return Balance{}, err
	}

	balances, err := ca.client.QueryBalanceWithAddress(acc.String())
	if err != nil {
		return Balance{}, err
	}

	// TODO @renaynay: will `celestia` balance always be first?
	return balances[0], nil
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
