package state

import (
	"fmt"
	"strings"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	coretypes "github.com/tendermint/tendermint/types"

	squareblob "github.com/celestiaorg/go-square/blob"
)

// Balance is an alias to the Coin type from Cosmos-SDK.
type Balance = sdk.Coin

// Tx is an alias to the Tx type from celestia-core.
type Tx = coretypes.Tx

// TxResponse is an alias to the TxResponse type from Cosmos-SDK.
type TxResponse = sdk.TxResponse

// Address is an alias to the Address type from Cosmos-SDK. It is embedded into a struct to provide
// a non-interface type for JSON serialization.
type Address struct {
	sdk.Address
}

// Blob is an alias of Blob from go-square.
type Blob = squareblob.Blob

// ValAddress is an alias to the ValAddress type from Cosmos-SDK.
type ValAddress = sdk.ValAddress

// AccAddress is an alias to the AccAddress type from Cosmos-SDK.
type AccAddress = sdk.AccAddress

// Int is an alias to the Int type from Cosmos-SDK.
type Int = math.Int

func (a *Address) UnmarshalJSON(data []byte) error {
	// To convert the string back to a concrete type, we have to determine the correct implementation
	var addr AccAddress
	addrString := strings.Trim(string(data), "\"")
	addr, err := sdk.AccAddressFromBech32(addrString)
	if err != nil {
		// first check if it is a validator address and can be converted
		valAddr, err := sdk.ValAddressFromBech32(addrString)
		if err != nil {
			return fmt.Errorf("address must be a valid account or validator address: %w", err)
		}
		a.Address = valAddr
		return nil
	}

	a.Address = addr
	return nil
}

func (a Address) MarshalJSON() ([]byte, error) {
	// The address is marshaled into a simple string value
	return []byte("\"" + a.Address.String() + "\""), nil
}
