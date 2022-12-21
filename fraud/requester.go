package fraud

import (
	"context"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"

	"github.com/celestiaorg/go-libp2p-messenger/serde"

	pb "github.com/celestiaorg/celestia-node/fraud/pb"
)

const (
	// writeDeadline sets timeout for sending messages to the stream
	writeDeadline = time.Second * 5
	// readDeadline sets timeout for reading messages from the stream
	readDeadline = time.Minute
)

func (f *ProofService) requestProofs(
	ctx context.Context,
	id protocol.ID,
	pid peer.ID,
	proofTypes []string,
) ([]*pb.ProofResponse, error) {
	msg := &pb.FraudMessageRequest{RequestedProofType: proofTypes}
	stream, err := f.host.NewStream(ctx, pid, id)
	if err != nil {
		return nil, err
	}

	if err = stream.SetWriteDeadline(time.Now().Add(writeDeadline)); err != nil {
		log.Warn(err)
	}
	_, err = serde.Write(stream, msg)
	if err != nil {
		stream.Reset() //nolint:errcheck
		return nil, err
	}
	if err = stream.CloseWrite(); err != nil {
		log.Warn(err)
	}
	if err = stream.SetReadDeadline(time.Now().Add(readDeadline)); err != nil {
		log.Warn(err)
	}
	resp := &pb.FraudMessageResponse{}
	_, err = serde.Read(stream, resp)
	if err != nil {
		stream.Reset() //nolint:errcheck
		return nil, err
	}
	return resp.Proofs, stream.Close()
}
