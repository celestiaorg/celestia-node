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

// WithTxWorkerAccounts configures the CoreAccessor to manage the provided number of
// worker accounts via the TxClient.
//   - Value of 0 submits transactions immediately (without a submission queue).
//   - Value of 1 uses synchronous submission (submission queue with default
//     signer as author of transactions).
//   - Value of > 1 uses parallel submission (submission queue with several accounts
//     submitting blobs). Parallel submission is not guaranteed to include blobs
//     in the same order as they were submitted.
func WithTxWorkerAccounts(workerAccounts int) Option {
	return func(ca *CoreAccessor) {
		ca.txWorkerAccounts = workerAccounts
	}
}
