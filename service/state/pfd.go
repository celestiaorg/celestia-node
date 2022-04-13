package state

import (
	"context"
	"fmt"

	"github.com/cosmos/cosmos-sdk/x/auth/signing"

	apptypes "github.com/celestiaorg/celestia-app/x/payment/types"
	"github.com/celestiaorg/nmt/namespace"
)

var (
	// shareSizes includes all the possible share sizes of the given data
	// that the signer must sign over.
	shareSizes = []uint64{16, 32, 64, 128}
)

// PayForData is an alias to the PayForMessage packet.
type PayForData = apptypes.MsgWirePayForMessage

// SubmitPayForData builds, signs and submits a PayForData message.
func (s *Service) SubmitPayForData(
	ctx context.Context,
	nID namespace.ID,
	data []byte,
	gasLim uint64,
	maxSize uint64,
) (*TxResponse, error) {
	pfd, err := s.BuildPayForData(ctx, nID, data, gasLim, maxSize)
	if err != nil {
		return nil, err
	}

	signed, err := s.SignPayForData(pfd, apptypes.SetGasLimit(gasLim))
	if err != nil {
		return nil, err
	}

	signer := s.accessor.KeyringSigner()
	rawTx, err := signer.EncodeTx(signed)
	if err != nil {
		return nil, err
	}

	return s.accessor.SubmitTx(ctx, rawTx)
}

// BuildPayForData builds a PayForData message.
func (s *Service) BuildPayForData(
	ctx context.Context,
	nID namespace.ID,
	message []byte,
	gasLim uint64,
	maxSize uint64,
) (*PayForData, error) {
	// create the raw WirePayForMessage transaction
	wpfmMsg, err := apptypes.NewWirePayForMessage(nID, message, shareSizes...)
	if err != nil {
		return nil, err
	}
	// TODO @renaynay: as a result of `padMessage`, the following is observed:
	//  {wpfmMsg.MessageSize: 256,  len(message): 5}
	if wpfmMsg.GetMessageSize() > maxSize {
		return nil, fmt.Errorf("message size %d cannot exceed specified max size %d", wpfmMsg.GetMessageSize(), maxSize)
	}

	// get signer and conn info
	signer := s.accessor.KeyringSigner()
	conn, err := s.accessor.Conn()
	if err != nil {
		return nil, err
	}

	// query for account information necessary to sign a valid tx
	err = signer.QueryAccountNumber(ctx, conn)
	if err != nil {
		return nil, err
	}

	// generate the signatures for each `MsgPayForMessage` using the `KeyringSigner`,
	// then set the gas limit for the tx
	gasLimOption := apptypes.SetGasLimit(gasLim)
	err = wpfmMsg.SignShareCommitments(signer, gasLimOption)
	if err != nil {
		return nil, err
	}

	return wpfmMsg, nil
}

// SignPayForData signs the given PayForData message.
func (s *Service) SignPayForData(pfd *PayForData, gasLimOption apptypes.TxBuilderOption) (signing.Tx, error) {
	signer := s.accessor.KeyringSigner()
	// Build and sign the final `WirePayForMessage` tx that now contains the signatures
	// for potential `MsgPayForMessage`s
	return signer.BuildSignedTx(
		gasLimOption(signer.NewTxBuilder()),
		pfd,
	)
}
