package state

import (
	"encoding/json"
	"fmt"

	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	sdktypes "github.com/cosmos/cosmos-sdk/types"

	"github.com/celestiaorg/celestia-app/v3/app/grpc/gasestimation"
	"github.com/celestiaorg/celestia-app/v3/pkg/appconsts"
)

const (
	// DefaultGasPrice specifies the default gas price value to be used when the user
	// wants to use the global minimal gas price, which is fetched from the celestia-app.
	DefaultGasPrice float64 = -1.0

	DefaultMaxGasPrice = appconsts.DefaultMinGasPrice * 100
)

// NewTxConfig constructs a new TxConfig with the provided options.
// It starts with a DefaultGasPrice and then applies any additional
// options provided through the variadic parameter.
func NewTxConfig(opts ...ConfigOption) *TxConfig {
	options := &TxConfig{gasPrice: DefaultGasPrice, maxGasPrice: DefaultMaxGasPrice}
	for _, opt := range opts {
		opt(options)
	}
	return options
}

type TxPriority int

const (
	TxPriorityLow = iota + 1
	TxPriorityMedium
	TxPriorityHigh
)

func (t TxPriority) toApp() gasestimation.TxPriority {
	switch t {
	case TxPriorityLow:
		return gasestimation.TxPriority_TX_PRIORITY_LOW
	case TxPriorityMedium:
		return gasestimation.TxPriority_TX_PRIORITY_MEDIUM
	case TxPriorityHigh:
		return gasestimation.TxPriority_TX_PRIORITY_HIGH
	default:
		return gasestimation.TxPriority_TX_PRIORITY_UNSPECIFIED
	}
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
	// specifies the max gas price that user expects to pay for the transaction.
	maxGasPrice float64
	// 0 gas means users want us to calculate it for them.
	gas uint64
	// priority is the priority level of the requested gas price.
	// - Low: The gas price is the value at the end of the lowest 10% of gas prices from the last 5 blocks.
	// - Medium: The gas price is the mean of all gas prices from the last 5 blocks.
	// - High: The gas price is the price at the start of the top 10% of transactions’ gas prices from the last 5 blocks.
	// - Default: Medium.
	priority TxPriority
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

func (cfg *TxConfig) GasLimit() uint64     { return cfg.gas }
func (cfg *TxConfig) MaxGasPrice() float64 { return cfg.maxGasPrice }

func (cfg *TxConfig) KeyName() string { return cfg.keyName }

func (cfg *TxConfig) SignerAddress() string { return cfg.signerAddress }

func (cfg *TxConfig) FeeGranterAddress() string { return cfg.feeGranterAddress }

type jsonTxConfig struct {
	GasPrice          float64 `json:"gas_price,omitempty"`
	IsGasPriceSet     bool    `json:"is_gas_price_set,omitempty"`
	MaxGasPrice       float64 `json:"max_gas_price"`
	Gas               uint64  `json:"gas,omitempty"`
	TxPriority        int     `json:"tx_priority,omitempty"`
	KeyName           string  `json:"key_name,omitempty"`
	SignerAddress     string  `json:"signer_address,omitempty"`
	FeeGranterAddress string  `json:"fee_granter_address,omitempty"`
}

func (cfg *TxConfig) MarshalJSON() ([]byte, error) {
	jsonOpts := &jsonTxConfig{
		GasPrice:          cfg.gasPrice,
		IsGasPriceSet:     cfg.isGasPriceSet,
		MaxGasPrice:       cfg.maxGasPrice,
		Gas:               cfg.gas,
		TxPriority:        int(cfg.priority),
		KeyName:           cfg.keyName,
		SignerAddress:     cfg.signerAddress,
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

	cfg.gasPrice = jsonOpts.GasPrice
	cfg.isGasPriceSet = jsonOpts.IsGasPriceSet
	cfg.maxGasPrice = jsonOpts.MaxGasPrice
	cfg.gas = jsonOpts.Gas
	cfg.priority = TxPriority(jsonOpts.TxPriority)
	cfg.keyName = jsonOpts.KeyName
	cfg.signerAddress = jsonOpts.SignerAddress
	cfg.feeGranterAddress = jsonOpts.FeeGranterAddress
	return nil
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

// WithMaxGasPrice is an option that allows you to specify a `maxGasPrice` field.
func WithMaxGasPrice(gasPrice float64) ConfigOption {
	return func(cfg *TxConfig) {
		cfg.maxGasPrice = gasPrice
	}
}

// WithTxPriority is an option that allows you to specify a priority of the tx.
func WithTxPriority(priority int) ConfigOption {
	return func(cfg *TxConfig) {
		if priority >= TxPriorityLow && priority <= TxPriorityHigh {
			cfg.priority = TxPriority(priority)
		}
	}
}
