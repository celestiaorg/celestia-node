package state

import (
	sdk_client "github.com/cosmos/cosmos-sdk/client"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// Account is an alias to the Account interface from Cosmos-SDK.
type Account = sdk_client.Account

// Balance is an alias to the Coin type from Cosmos-SDK.
type Balance = sdk.Coin

// Msg is an alias to the Msg interface from Cosmos-SDK.
type Msg = sdk.Msg

// TxResponse is an alias to the TxResponse type from Cosmos-SDK.
type TxResponse = sdk.TxResponse
