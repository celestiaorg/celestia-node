package state

import (
	"fmt"

	sdk_errors "github.com/cosmos/cosmos-sdk/types/errors"
	sdk_abci "github.com/tendermint/tendermint/abci/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func sdkErrorToGRPCError(resp sdk_abci.ResponseQuery) error {
	switch codeToError(resp.Code) {
	case sdk_errors.ErrInvalidRequest:
		return status.Error(codes.InvalidArgument, resp.Log)
	case sdk_errors.ErrUnauthorized:
		return status.Error(codes.Unauthenticated, resp.Log)
	case sdk_errors.ErrKeyNotFound:
		return status.Error(codes.NotFound, resp.Log)
	default:
		return status.Error(codes.Unknown, resp.Log)
	}
}

func codeToError(code uint32) error {
	switch code {
	case 0:
		return nil
	case sdk_errors.ErrTxDecode.ABCICode():
		return sdk_errors.ErrTxDecode
	case sdk_errors.ErrUnauthorized.ABCICode():
		return sdk_errors.ErrUnauthorized
	case sdk_errors.ErrInsufficientFunds.ABCICode():
		return sdk_errors.ErrInsufficientFunds
	case sdk_errors.ErrInvalidAddress.ABCICode():
		return sdk_errors.ErrInvalidAddress
	case sdk_errors.ErrUnknownAddress.ABCICode():
		return sdk_errors.ErrUnknownAddress
	case sdk_errors.ErrInvalidCoins.ABCICode():
		return sdk_errors.ErrInvalidCoins
	case sdk_errors.ErrOutOfGas.ABCICode():
		return sdk_errors.ErrOutOfGas
	case sdk_errors.ErrInsufficientFee.ABCICode():
		return sdk_errors.ErrInsufficientFee
	case sdk_errors.ErrJSONMarshal.ABCICode():
		return sdk_errors.ErrJSONMarshal
	case sdk_errors.ErrJSONUnmarshal.ABCICode():
		return sdk_errors.ErrJSONUnmarshal
	case sdk_errors.ErrTxTooLarge.ABCICode():
		return sdk_errors.ErrTxTooLarge
	case sdk_errors.ErrorInvalidGasAdjustment.ABCICode():
		return sdk_errors.ErrorInvalidGasAdjustment
	case sdk_errors.ErrLogic.ABCICode():
		return sdk_errors.ErrLogic
	case sdk_errors.ErrNotFound.ABCICode():
		return sdk_errors.ErrNotFound
	case sdk_errors.ErrIO.ABCICode():
		return sdk_errors.ErrIO
	case sdk_errors.ErrInvalidGasLimit.ABCICode():
		return sdk_errors.ErrInvalidGasLimit
	default:
		return fmt.Errorf("unknown error code received: %d", code)
	}
}
