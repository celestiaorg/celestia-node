package state

type Option func(ca *CoreAccessor)

// WithEstimatorService indicates to the CoreAccessor to pass the given address
// for the estimator service to the TxClient to use for all gas price and usage
// estimation queries.
func WithEstimatorService(address string) Option {
	return func(ca *CoreAccessor) {
		ca.estimatorServiceAddr = address
	}
}
