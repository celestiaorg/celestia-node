package state

import (
	sdk_errors "github.com/cosmos/cosmos-sdk/types/errors"
	sdk_abci "github.com/tendermint/tendermint/abci/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func sdkErrorToGRPCError(resp sdk_abci.ResponseQuery) error {
	switch resp.Code {
	case sdk_errors.ErrInvalidRequest.ABCICode():
		return status.Error(codes.InvalidArgument, resp.Log)
	case sdk_errors.ErrUnauthorized.ABCICode():
		return status.Error(codes.Unauthenticated, resp.Log)
	case sdk_errors.ErrKeyNotFound.ABCICode():
		return status.Error(codes.NotFound, resp.Log)
	default:
		return status.Error(codes.Unknown, resp.Log)
	}
}
