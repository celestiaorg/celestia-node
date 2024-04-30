package options

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"

	sdktypes "github.com/cosmos/cosmos-sdk/types"

	"github.com/celestiaorg/celestia-app/pkg/user"
	apptypes "github.com/celestiaorg/celestia-app/x/blob/types"
)

const (
	// gasMultiplier is used to increase gas limit in case if tx has additional options.
	gasMultiplier = 1.1

	//	Since 0 is a valid fee input for the Tx, the default value is -1.
	defaultFeeAmount = -1
)

var (
	errNoGasProvided     = errors.New("gas limit was not set")
	errNoAddressProvided = errors.New("address was not set")
)

// TxOptions specifies additional options that will be applied to the Tx.
type TxOptions struct {
	// fee is private since it has to be set through `SetFeeAmount`
	fee      *Fee
	GasLimit uint64

	// Specifies the key that should be used to sign transactions.
	//  Should be a Bech32 string
	// NOTE: This `Account` should be available in the `Keystore`.
	Account string
	// Specifies the account that will pay for the transaction.
	//	//  Should be a Bech32 string
	Granter string
}

func DefaultTxOptions() *TxOptions {
	return &TxOptions{
		fee:      DefaultFee(),
		GasLimit: 0,
		Account:  "",
	}
}

type jsonTxOptions struct {
	Fee      *Fee   `json:"fee,omitempty"`
	GasLimit uint64 `json:"gasLimit,omitempty"`
	Account  string `json:"account,omitempty"`
	Granter  string `json:"granter,omitempty"`
}

func (options *TxOptions) MarshalJSON() ([]byte, error) {
	jsonOpts := &jsonTxOptions{
		Fee:      options.fee,
		GasLimit: options.GasLimit,
		Account:  options.Account,
		Granter:  options.Granter,
	}
	return json.Marshal(jsonOpts)
}

func (options *TxOptions) UnmarshalJSON(data []byte) error {
	var jsonOpts jsonTxOptions
	err := json.Unmarshal(data, &jsonOpts)
	if err != nil {
		return fmt.Errorf("unmarshalling TxOptions:%w", err)
	}

	options.fee = jsonOpts.Fee
	options.GasLimit = jsonOpts.GasLimit
	options.Account = jsonOpts.Account
	options.Granter = jsonOpts.Granter
	return nil
}

// SetFeeAmount sets fee for the transaction.
func (options *TxOptions) SetFeeAmount(amount int64) {
	if amount >= 0 {
		options.fee.Amount = amount
		options.fee.isSet = true
	}
}

// CalculateFee calculates fee amount as a `user.TxOption` that can be applied to the transactions,
// based on the `minGasPrice` and `GasLimit`.
// NOTE: it is very important to ensure that gas limit was set.
func (options *TxOptions) CalculateFee(minGasPrice float64) error {
	if options.GasLimit == 0 {
		return errNoGasProvided
	}
	if minGasPrice < 0 {
		return errors.New("minimum gas price should be equal or greater than zero")
	}
	options.fee.Amount = int64(math.Ceil(minGasPrice * float64(options.GasLimit)))
	options.fee.isSet = true
	return nil
}

func (options *TxOptions) GetFee() uint64 {
	return uint64(options.fee.Amount)
}

func (options *TxOptions) IsFeeSet() bool {
	return options.fee.isSet
}

// EstimateGas returns a gas limit as a `user.TxOption` that can be applied to the transactions.
// GasLimit will be estimated in case it has not been set.
// NOTE: final result of the estimation will be multiplied by the `gasMultiplier`(1.1) to cover additional сosts for the
// options of the Tx, as not all options are counted in the `EstimateGas`
func (options *TxOptions) EstimateGas(ctx context.Context, client *user.TxClient, msg sdktypes.Msg) error {
	if options.GasLimit == 0 {
		// Gas estimation depends on the amount of bytes of the original TX, including all options(Fee, Granter, etc.),
		// so, it is very important to pass ALL options that will be applied to the tx.
		// 1utia as fee is ok at this point(as fee may not be calculated yet).
		gasLimit, err := client.EstimateGas(ctx, []sdktypes.Msg{msg}, user.SetFee(1))
		if err != nil {
			return fmt.Errorf("estimating gas: %w", err)
		}
		options.GasLimit = uint64(float64(gasLimit) * gasMultiplier)
	}
	return nil
}

// EstimateGasForBlobs returns a gas limit as a `user.TxOption` that can be applied to the `MsgPayForBlob` transactions.
// NOTE: final result of the estimation will be multiplied by the `gasMultiplier`(1.1)
// to cover additional options of the Tx.
func (options *TxOptions) EstimateGasForBlobs(blobSizes []uint32) {
	if options.GasLimit == 0 {
		gasLimit := apptypes.DefaultEstimateGas(blobSizes)
		options.GasLimit = uint64(float64(gasLimit) * gasMultiplier)
	}
}

func (options *TxOptions) GetAccount() (sdktypes.AccAddress, error) {
	if options.Account == "" {
		return nil, errNoAddressProvided
	}
	return parseAccAddressFromString(options.Account)
}

func (options *TxOptions) GetGranter() (sdktypes.AccAddress, error) {
	if options.Granter == "" {
		return nil, fmt.Errorf("granter %s", errNoAddressProvided.Error())
	}

	return parseAccAddressFromString(options.Granter)
}

type Fee struct {
	Amount int64
	isSet  bool
}

// DefaultFee creates a Fee struct with the default value of fee amount.
func DefaultFee() *Fee {
	return &Fee{
		Amount: defaultFeeAmount,
	}
}

type jsonFee struct {
	Amount int64 `json:"amount,omitempty"`
	IsSet  bool  `json:"isSet,omitempty"`
}

func (f *Fee) MarshalJSON() ([]byte, error) {
	fee := jsonFee{
		Amount: f.Amount,
		IsSet:  f.isSet,
	}
	return json.Marshal(fee)
}

func (f *Fee) UnmarshalJSON(data []byte) error {
	var fee jsonFee
	err := json.Unmarshal(data, &fee)
	if err != nil {
		return err
	}

	f.Amount = fee.Amount
	f.isSet = fee.IsSet
	if !f.isSet {
		f.Amount = -1
	}
	return nil
}

func parseAccAddressFromString(addrStr string) (sdktypes.AccAddress, error) {
	addrString := strings.Trim(addrStr, "\"")
	return sdktypes.AccAddressFromBech32(addrString)
}
