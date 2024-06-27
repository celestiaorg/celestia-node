package state

import (
	"context"
	"encoding/json"
	"fmt"
	"math"

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

func NewTxOptions(attributes ...Attribute) *TxOptions {
	options := &TxOptions{gasPrice: DefaultPrice}
	for _, attr := range attributes {
		attr(options)
	}
	return options
}

// TxOptions specifies additional options that will be applied to the Tx.
// Implements `Option` interface.
type TxOptions struct {
	// Specifies the address from the keystore that will sign transactions.
	// NOTE: Only `accountAddress` or `KeyName` should be passed.
	// accountAddress is a primary options. This means If both the address and the key are specified,
	// the address field will take priority.
	signerAddress string
	// Specifies the key from the keystore associated with an account that
	// will be used to sign transactions.
	// NOTE: This `Account` must be available in the `Keystore`.
	keyName string
	// gasPrice represents the amount to be paid per gas unit.
	// Negative gasPrice means user want us to use the minGasPrice
	// defined in the node.
	gasPrice float64
	// since gasPrice can be 0, it is necessary to understand that user explicitly set it.
	isGasPriceSet bool
	// 0 gas means users want us to calculate it for them.
	gas uint64

	// Specifies the account that will pay for the transaction.
	// Input format Bech32.
	feeGranterAddress string
}

func (options *TxOptions) GasPrice() float64 {
	if !options.isGasPriceSet {
		return DefaultPrice
	}
	return options.gasPrice
}

func (options *TxOptions) GasLimit() uint64 { return options.gas }

func (options *TxOptions) KeyName() string { return options.keyName }

func (options *TxOptions) SignerAddress() string { return options.signerAddress }

func (options *TxOptions) FeeGranterAddress() string { return options.feeGranterAddress }

type jsonTxOptions struct {
	GasPrice          float64 `json:"gas_price,omitempty"`
	IsGasPriceSet     bool    `json:"is_gas_price_set,omitempty"`
	Gas               uint64  `json:"gas,omitempty"`
	KeyName           string  `json:"key_name,omitempty"`
	SignerAddress     string  `json:"signer_address,omitempty"`
	FeeGranterAddress string  `json:"fee_granter_address,omitempty"`
}

func (options *TxOptions) MarshalJSON() ([]byte, error) {
	jsonOpts := &jsonTxOptions{
		SignerAddress:     options.signerAddress,
		KeyName:           options.keyName,
		GasPrice:          options.gasPrice,
		IsGasPriceSet:     options.isGasPriceSet,
		Gas:               options.gas,
		FeeGranterAddress: options.feeGranterAddress,
	}
	return json.Marshal(jsonOpts)
}

func (options *TxOptions) UnmarshalJSON(data []byte) error {
	var jsonOpts jsonTxOptions
	err := json.Unmarshal(data, &jsonOpts)
	if err != nil {
		return fmt.Errorf("unmarshalling TxOptions: %w", err)
	}

	options.keyName = jsonOpts.KeyName
	options.signerAddress = jsonOpts.SignerAddress
	options.gasPrice = jsonOpts.GasPrice
	options.isGasPriceSet = jsonOpts.IsGasPriceSet
	options.gas = jsonOpts.Gas
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
	return sdktypes.AccAddressFromBech32(addrStr)
}

// Attribute is the functional option that is applied to the TxOption instance
// to configure parameters.
type Attribute func(options *TxOptions)

// WithGasPrice is an attribute that allows to specify a GasPrice, which is needed
// to calculate the fee. In case GasPrice is not specified, the global GasPrice fetched from
// celestia-app will be used.
func WithGasPrice(gasPrice float64) Attribute {
	return func(options *TxOptions) {
		if gasPrice >= 0 {
			options.gasPrice = gasPrice
			options.isGasPriceSet = true
		}
	}
}

// WithGas is an attribute that allows to specify Gas.
// Gas will be calculated in case it wasn't specified.
func WithGas(gas uint64) Attribute {
	return func(options *TxOptions) {
		options.gas = gas
	}
}

// WithKeyName is an attribute that allows you to specify an KeyName, which is needed to
// sign the transaction. This key should be associated with the address and stored
// locally in the key store. Default Account will be used in case it wasn't specified.
func WithKeyName(key string) Attribute {
	return func(options *TxOptions) {
		options.keyName = key
	}
}

// WithSignerAddress is an attribute that allows you to specify an address, that will sign the transaction.
// This address must be stored locally in the key store. Default signerAddress will be used in case it wasn't specified.
func WithSignerAddress(address string) Attribute {
	return func(options *TxOptions) {
		options.signerAddress = address
	}
}

// WithFeeGranterAddress is an attribute that allows you to specify a GranterAddress to pay the fees.
func WithFeeGranterAddress(granter string) Attribute {
	return func(options *TxOptions) {
		options.feeGranterAddress = granter
	}
}
