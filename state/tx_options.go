package state

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	sdktypes "github.com/cosmos/cosmos-sdk/types"

	"github.com/celestiaorg/celestia-app/pkg/user"
	apptypes "github.com/celestiaorg/celestia-app/x/blob/types"
)

const (
	DefaultPrice float64 = -1.0
	// gasMultiplier is used to increase gas limit in case if tx has additional options.
	gasMultiplier = 1.1
)

// Options is an interface that represents a set of methods required for managing
// transaction options.
type Options interface {
	GasPrice() float64
	GasLimit() uint64
	AccountKey() string
	FeeGranterAddress() string

	json.Marshaler
	json.Unmarshaler
}

func NewTxOptions(attributes ...Attribute) Options {
	options := &txOptions{gasPrice: DefaultPrice}
	for _, attr := range attributes {
		attr(options)
	}
	return options
}

// txOptions specifies additional options that will be applied to the Tx.
// Implements `Option` interface.
type txOptions struct {
	// gasPrice represents the amount to be paid per gas unit.
	// Negative gasPrice means user want us to use the minGasPrice
	// defined in the node.
	gasPrice float64
	// 0 gas means users want us to calculate it for them.
	gas uint64

	// Specifies the key from the keystore associated with an account that
	// will be used to sign transactions.
	// NOTE: This `Account` must be available in the `Keystore`.
	accountKey string
	// Specifies the account that will pay for the transaction.
	// Input format Bech32.
	feeGranterAddress string
}

func (options *txOptions) GasPrice() float64 { return options.gasPrice }

func (options *txOptions) GasLimit() uint64 { return options.gas }

func (options *txOptions) AccountKey() string { return options.accountKey }

func (options *txOptions) FeeGranterAddress() string { return options.feeGranterAddress }

type jsonTxOptions struct {
	GasPrice          float64 `json:"gas_price,omitempty"`
	Gas               uint64  `json:"gas,omitempty"`
	AccountKey        string  `json:"account_key,omitempty"`
	FeeGranterAddress string  `json:"fee_granter_address,omitempty"`
}

func (options *txOptions) MarshalJSON() ([]byte, error) {
	jsonOpts := &jsonTxOptions{
		GasPrice:          options.gasPrice,
		Gas:               options.gas,
		AccountKey:        options.accountKey,
		FeeGranterAddress: options.feeGranterAddress,
	}
	return json.Marshal(jsonOpts)
}

func (options *txOptions) UnmarshalJSON(data []byte) error {
	var jsonOpts jsonTxOptions
	err := json.Unmarshal(data, &jsonOpts)
	if err != nil {
		return fmt.Errorf("unmarshalling TxOptions: %w", err)
	}

	options.gasPrice = jsonOpts.GasPrice
	options.gas = jsonOpts.Gas
	options.accountKey = jsonOpts.AccountKey
	options.feeGranterAddress = jsonOpts.FeeGranterAddress
	return nil
}

// calculateFee calculates fee amount based on the `minGasPrice` and `Gas`.
func calculateFee(gas uint64, gasPrice float64) int64 {
	fee := int64(math.Ceil(gasPrice * float64(gas)))
	return fee
}

// estimateGas estimates gas in case it has not been set.
// NOTE: final result of the estimation will be multiplied by the `gasMultiplier`(1.1) to cover additional costs.
func estimateGas(ctx context.Context, client *user.TxClient, msg sdktypes.Msg) (uint64, error) {
	// set fee as 1utia helps to simulate the tx more reliably.
	gas, err := client.EstimateGas(ctx, []sdktypes.Msg{msg}, user.SetFee(1))
	if err != nil {
		return 0, fmt.Errorf("estimating gas: %w", err)
	}
	return uint64(float64(gas) * gasMultiplier), nil
}

// estimateGasForBlobs returns a gas limit as a `user.TxOption` that can be applied to the `MsgPayForBlob` transactions.
// NOTE: final result of the estimation will be multiplied by the `gasMultiplier`(1.1)
// to cover additional options of the Tx.
func estimateGasForBlobs(blobSizes []uint32) uint64 {
	gas := apptypes.DefaultEstimateGas(blobSizes)
	return uint64(float64(gas) * gasMultiplier)
}

func parseAccountKey(kr keyring.Keyring, accountKey string) (sdktypes.AccAddress, error) {
	rec, err := kr.Key(accountKey)
	if err != nil {
		return nil, fmt.Errorf("getting account key: %w", err)
	}
	return rec.GetAddress()
}

func parseAccAddressFromString(addrStr string) (sdktypes.AccAddress, error) {
	addrString := strings.Trim(addrStr, "\"")
	return sdktypes.AccAddressFromBech32(addrString)
}

// Attribute is the functional option that is applied to the TxOption instance
// to configure parameters.
type Attribute func(options *txOptions)

// WithGasPrice is an attribute that allows to specify a GasPrice
func WithGasPrice(gasPrice float64) Attribute {
	return func(options *txOptions) {
		options.gasPrice = gasPrice
	}
}

// WithGas is an attribute that allows to specify a Gas
func WithGas(gas uint64) Attribute {
	return func(options *txOptions) {
		options.gas = gas
	}
}

// WithAccountKey is an attribute that allows to specify an AccountKey
func WithAccountKey(key string) Attribute {
	return func(options *txOptions) {
		options.accountKey = key
	}
}

// WithFeeGranterAddress is an attribute that allows to specify a GranterAddress
func WithFeeGranterAddress(granter string) Attribute {
	return func(options *txOptions) {
		options.feeGranterAddress = granter
	}
}
