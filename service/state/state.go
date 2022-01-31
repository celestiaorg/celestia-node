package state

import (
	sdk_client "github.com/cosmos/cosmos-sdk/client"
	sdk "github.com/cosmos/cosmos-sdk/types"
	coretypes "github.com/tendermint/tendermint/types"
)

// Account is an alias to the Account interface from Cosmos-SDK.
type Account = sdk_client.Account

// Balance is an alias to the Coin type from Cosmos-SDK.
type Balance = sdk.Coin

// Msg is an alias to the Msg interface from Cosmos-SDK. // TODO @renaynay: do we need this?
type Msg = sdk.Msg

// Tx is an alias to the Tx type from celestia-core.
type Tx = coretypes.Tx

// TxResponse is an alias to the TxResponse type from Cosmos-SDK.
type TxResponse = sdk.TxResponse
