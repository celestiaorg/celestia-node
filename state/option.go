package state

import (
	"google.golang.org/grpc"
)

type Option func(ca *CoreAccessor)

// WithEstimatorService indicates to the CoreAccessor to pass the given address
// for the estimator service to the TxClient to use for all gas price and usage
// estimation queries.
func WithEstimatorService(address string) Option {
	return func(ca *CoreAccessor) {
		ca.estimatorServiceAddr = address
	}
}

// WithEstimatorServiceTLS indicates to the CoreAccessor to use TLS for the
// estimator service connection.
func WithEstimatorServiceTLS() Option {
	return func(ca *CoreAccessor) {
		ca.estimatorServiceTLS = true
	}
}

func WithAdditionalCoreEndpoints(conns []*grpc.ClientConn) Option {
	return func(ca *CoreAccessor) {
		ca.coreConns = append(ca.coreConns, conns...)
	}
}
