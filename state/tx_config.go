package state

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	sdktypes "github.com/cosmos/cosmos-sdk/types"

	"github.com/celestiaorg/celestia-app/v2/pkg/user"
	apptypes "github.com/celestiaorg/celestia-app/v2/x/blob/types"
)

const (
	// DefaultGasPrice specifies the default gas price value to be used when the user
	// wants to use the global minimal gas price, which is fetched from the celestia-app.
	DefaultGasPrice float64 = -1.0
	// gasMultiplier is used to increase gas limit in case if tx has additional cfg.
	gasMultiplier = 1.1
)

// NewTxConfig constructs a new TxConfig with the provided options.
// It starts with a DefaultGasPrice and then applies any additional
// options provided through the variadic parameter.
func NewTxConfig(opts ...ConfigOption) *TxConfig {
	options := &TxConfig{gasPrice: DefaultGasPrice}
	for _, opt := range opts {
		opt(options)
	}
	return options
}

// TxConfig specifies additional options that will be applied to the Tx.
type TxConfig struct {
	// Specifies the address from the keystore that will sign transactions.
	// NOTE: Only `signerAddress` or `KeyName` should be passed.
	// signerAddress is a primary cfg. This means If both the address and the key are specified,
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

func (cfg *TxConfig) GasPrice() float64 {
	if !cfg.isGasPriceSet {
		return DefaultGasPrice
	}
	return cfg.gasPrice
}

func (cfg *TxConfig) GasLimit() uint64 { return cfg.gas }

func (cfg *TxConfig) KeyName() string { return cfg.keyName }

func (cfg *TxConfig) SignerAddress() string { return cfg.signerAddress }

func (cfg *TxConfig) FeeGranterAddress() string { return cfg.feeGranterAddress }

type jsonTxConfig struct {
	GasPrice          float64 `json:"gas_price,omitempty"`
	IsGasPriceSet     bool    `json:"is_gas_price_set,omitempty"`
	Gas               uint64  `json:"gas,omitempty"`
	KeyName           string  `json:"key_name,omitempty"`
	SignerAddress     string  `json:"signer_address,omitempty"`
	FeeGranterAddress string  `json:"fee_granter_address,omitempty"`
}

func (cfg *TxConfig) MarshalJSON() ([]byte, error) {
	jsonOpts := &jsonTxConfig{
		SignerAddress:     cfg.signerAddress,
		KeyName:           cfg.keyName,
		GasPrice:          cfg.gasPrice,
		IsGasPriceSet:     cfg.isGasPriceSet,
		Gas:               cfg.gas,
		FeeGranterAddress: cfg.feeGranterAddress,
	}
	return json.Marshal(jsonOpts)
}

func (cfg *TxConfig) UnmarshalJSON(data []byte) error {
	var jsonOpts jsonTxConfig
	err := json.Unmarshal(data, &jsonOpts)
	if err != nil {
		return fmt.Errorf("unmarshalling TxConfig: %w", err)
	}

	cfg.keyName = jsonOpts.KeyName
	cfg.signerAddress = jsonOpts.SignerAddress
	cfg.gasPrice = jsonOpts.GasPrice
	cfg.isGasPriceSet = jsonOpts.IsGasPriceSet
	cfg.gas = jsonOpts.Gas
	cfg.feeGranterAddress = jsonOpts.FeeGranterAddress
	return nil
}

// estimateGas estimates gas in case it has not been set.
// NOTE: final result of the estimation will be multiplied by the `gasMultiplier`(1.1) to cover
// additional costs.
func estimateGas(ctx context.Context, client *user.TxClient, msg sdktypes.Msg) (uint64, error) {
	// set fee as 1utia helps to simulate the tx more reliably.
	gas, err := client.EstimateGas(ctx, []sdktypes.Msg{msg}, user.SetFee(1))
	if err != nil {
		return 0, fmt.Errorf("estimating gas: %w", err)
	}
	return uint64(float64(gas) * gasMultiplier), nil
}

// estimateGasForBlobs returns a gas limit that can be applied to the `MsgPayForBlob` transactions.
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

// ConfigOption is the functional option that is applied to the TxConfig instance
// to configure parameters.
type ConfigOption func(cfg *TxConfig)

// WithGasPrice is an option that allows to specify a GasPrice, which is needed
// to calculate the fee. In case GasPrice is not specified, the global GasPrice fetched from
// celestia-app will be used.
func WithGasPrice(gasPrice float64) ConfigOption {
	return func(cfg *TxConfig) {
		if gasPrice >= 0 {
			cfg.gasPrice = gasPrice
			cfg.isGasPriceSet = true
		}
	}
}

// WithGas is an option that allows to specify Gas.
// Gas will be calculated in case it wasn't specified.
func WithGas(gas uint64) ConfigOption {
	return func(cfg *TxConfig) {
		cfg.gas = gas
	}
}

// WithKeyName is an option that allows you to specify an KeyName, which is needed to
// sign the transaction. This key should be associated with the address and stored
// locally in the key store. Default Account will be used in case it wasn't specified.
func WithKeyName(key string) ConfigOption {
	return func(cfg *TxConfig) {
		cfg.keyName = key
	}
}

// WithSignerAddress is an option that allows you to specify an address, that will sign the
// transaction. This address must be stored locally in the key store. Default signerAddress will be
// used in case it wasn't specified.
func WithSignerAddress(address string) ConfigOption {
	return func(cfg *TxConfig) {
		cfg.signerAddress = address
	}
}

// WithFeeGranterAddress is an option that allows you to specify a GranterAddress to pay the fees.
func WithFeeGranterAddress(granter string) ConfigOption {
	return func(cfg *TxConfig) {
		cfg.feeGranterAddress = granter
	}
}
