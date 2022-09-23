package state

import (
	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	coretypes "github.com/tendermint/tendermint/types"
)

// Balance is an alias to the Coin type from Cosmos-SDK.
type Balance = sdk.Coin

// Tx is an alias to the Tx type from celestia-core.
type Tx = coretypes.Tx

// TxResponse is an alias to the TxResponse type from Cosmos-SDK.
type TxResponse = sdk.TxResponse

// Address is an alias to the Address type from Cosmos-SDK.
type Address = sdk.Address

// ValAddress is an alias to the ValAddress type from Cosmos-SDK.
type ValAddress = sdk.ValAddress

// AccAddress is an alias to the AccAddress type from Cosmos-SDK.
type AccAddress = sdk.AccAddress

// Int is an alias to the Int type from Cosmos-SDK.
type Int = math.Int
