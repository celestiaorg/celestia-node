package state

type Option func(ca *CoreAccessor)

// WithEstimatorService TODO @renaynay
func WithEstimatorService(address string) Option {
	return func(ca *CoreAccessor) {
		ca.estimatorServiceAddr = address
	}
}
