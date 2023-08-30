package blob

import "cosmossdk.io/math"

// Option is a function that allows to configure the Blob Service requests.
type Option func(*Service)

// feeParams contains the information about fee and gasLimit price.
type feeParams struct {
	fee    math.Int
	gasLim uint64
}

// WithFee allows to configure the fee.
func WithFee(fee int64) Option {
	return func(s *Service) {
		s.feeParams.fee = math.NewInt(fee)

	}
}

// WithGasLimit sets the maximum amount of gas that could be paid.
func WithGasLimit(gasLim uint64) Option {
	return func(s *Service) {
		s.feeParams.gasLim = gasLim
	}
}
