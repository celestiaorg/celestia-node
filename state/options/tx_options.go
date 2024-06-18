package options

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	sdktypes "github.com/cosmos/cosmos-sdk/types"

	"github.com/celestiaorg/celestia-app/pkg/user"
	apptypes "github.com/celestiaorg/celestia-app/x/blob/types"
)

const (
	// gasMultiplier is used to increase gas limit in case if tx has additional options.
	gasMultiplier = 1.1

	DefaultPrice float64 = -1.0
)

var (
	errNoGasProvided     = errors.New("gas limit was not set")
	errNoAddressProvided = errors.New("address was not set")
)

// TxOptions specifies additional options that will be applied to the Tx.
type TxOptions struct {
	// GasPrice represents the amount to be paid per gas unit.
	// Negative GasPrice means user want us to use a minGasPrice,
	// stored in node.
	gasPrice *GasPrice
	// 0 Gas means users want us to calculate it for them.
	Gas uint64

	// Specifies the key from the keystore associated with an account that
	// will be used to sign transactions.
	// NOTE: This `Account` must be available in the `Keystore`.
	AccountKey string
	// Specifies the account that will pay for the transaction.
	// Input format Bech32.
	FeeGranterAddress string
}

func DefaultTxOptions() *TxOptions {
	return &TxOptions{
		gasPrice: DefaultGasPrice(),
	}
}

type jsonTxOptions struct {
	GasPrice          *GasPrice `json:"fee,omitempty"`
	Gas               uint64    `json:"gas,omitempty"`
	AccountKey        string    `json:"account,omitempty"`
	FeeGranterAddress string    `json:"granter,omitempty"`
}

func (options *TxOptions) MarshalJSON() ([]byte, error) {
	jsonOpts := &jsonTxOptions{
		GasPrice:          options.gasPrice,
		Gas:               options.Gas,
		AccountKey:        options.AccountKey,
		FeeGranterAddress: options.FeeGranterAddress,
	}
	return json.Marshal(jsonOpts)
}

func (options *TxOptions) UnmarshalJSON(data []byte) error {
	var jsonOpts jsonTxOptions
	err := json.Unmarshal(data, &jsonOpts)
	if err != nil {
		return fmt.Errorf("unmarshalling TxOptions:%w", err)
	}

	options.gasPrice = jsonOpts.GasPrice
	options.Gas = jsonOpts.Gas
	options.AccountKey = jsonOpts.AccountKey
	options.FeeGranterAddress = jsonOpts.FeeGranterAddress
	return nil
}

// SetGasPrice sets fee for the transaction.
func (options *TxOptions) SetGasPrice(price float64) {
	if options.gasPrice == nil {
		options.gasPrice = DefaultGasPrice()
	}

	if price >= 0 {
		options.gasPrice.Price = price
		options.gasPrice.isSet = true
	}
}

func (options *TxOptions) GetGasPrice() float64 {
	if options.gasPrice == nil {
		options.gasPrice = DefaultGasPrice()
	}
	return options.gasPrice.Price
}

// CalculateFee calculates fee amount based on the `minGasPrice` and `Gas`.
// NOTE: Gas can't be 0.
func (options *TxOptions) CalculateFee() (int64, error) {
	if options.Gas == 0 {
		return 0, errNoGasProvided
	}
	if options.gasPrice == nil {
		options.gasPrice = DefaultGasPrice()
	}

	if options.gasPrice.Price < 0 {
		return 0, errors.New(" gas price can't be negative")
	}
	fee := int64(math.Ceil(options.gasPrice.Price * float64(options.Gas)))
	return fee, nil
}

// EstimateGas estimates gas in case it has not been set.
// NOTE: final result of the estimation will be multiplied by the `gasMultiplier`(1.1) to cover additional costs.
func (options *TxOptions) EstimateGas(ctx context.Context, client *user.TxClient, msg sdktypes.Msg) error {
	if options.Gas == 0 {
		// set fee as 1utia helps to simulate the tx more reliably.
		gas, err := client.EstimateGas(ctx, []sdktypes.Msg{msg}, user.SetFee(1))
		if err != nil {
			return fmt.Errorf("estimating gas: %w", err)
		}
		options.Gas = uint64(float64(gas) * gasMultiplier)
	}
	return nil
}

// EstimateGasForBlobs returns a gas limit as a `user.TxOption` that can be applied to the `MsgPayForBlob` transactions.
// NOTE: final result of the estimation will be multiplied by the `gasMultiplier`(1.1)
// to cover additional options of the Tx.
func (options *TxOptions) EstimateGasForBlobs(blobSizes []uint32) {
	if options.Gas == 0 {
		gas := apptypes.DefaultEstimateGas(blobSizes)
		options.Gas = uint64(float64(gas) * gasMultiplier)
	}
}

// GetSigner retrieves the keystore by the provided account name and returns the account address.
func (options *TxOptions) GetSigner(kr keyring.Keyring) (sdktypes.AccAddress, error) {
	if options.AccountKey == "" {
		return nil, errNoAddressProvided
	}
	rec, err := kr.Key(options.AccountKey)
	if err != nil {
		return nil, fmt.Errorf("getting account key: %w", err)
	}
	return rec.GetAddress()
}

// GetFeeGranterAddress converts provided granter address to the cosmos-sdk `AccAddress`
func (options *TxOptions) GetFeeGranterAddress() (sdktypes.AccAddress, error) {
	if options.FeeGranterAddress == "" {
		return nil, fmt.Errorf("granter address %s", errNoAddressProvided.Error())
	}

	return parseAccAddressFromString(options.FeeGranterAddress)
}

type GasPrice struct {
	Price float64
	isSet bool
}

// DefaultGasPrice creates a Fee struct with the default value of fee amount.
func DefaultGasPrice() *GasPrice {
	return &GasPrice{
		Price: DefaultPrice,
	}
}

type jsonGasPrice struct {
	Price float64 `json:"price,omitempty"`
	IsSet bool    `json:"isSet,omitempty"`
}

func (g *GasPrice) MarshalJSON() ([]byte, error) {
	fee := jsonGasPrice{
		Price: g.Price,
		IsSet: g.isSet,
	}
	return json.Marshal(fee)
}

func (g *GasPrice) UnmarshalJSON(data []byte) error {
	var gasPrice jsonGasPrice
	err := json.Unmarshal(data, &gasPrice)
	if err != nil {
		return err
	}

	g.Price = gasPrice.Price
	g.isSet = gasPrice.IsSet
	if !g.isSet {
		g.Price = -1
	}
	return nil
}

func parseAccAddressFromString(addrStr string) (sdktypes.AccAddress, error) {
	addrString := strings.Trim(addrStr, "\"")
	return sdktypes.AccAddressFromBech32(addrString)
}
