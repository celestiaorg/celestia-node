package fraud

import (
	"context"

	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/celestiaorg/go-libp2p-messenger/serde"

	pb "github.com/celestiaorg/celestia-node/fraud/pb"
)

func requestProofs(
	ctx context.Context,
	host host.Host,
	pid peer.ID,
	proofTypes []pb.ProofType,
) ([]*pb.ProofResponse, error) {
	msg := &pb.FraudMessageRequest{RequestedProofType: proofTypes}
	stream, err := host.NewStream(ctx, pid, fraudProtocolID)
	if err != nil {
		return nil, err
	}

	_, err = serde.Write(stream, msg)
	if err != nil {
		stream.Reset() //nolint:errcheck
		return nil, err
	}
	resp := &pb.FraudMessageResponse{}
	_, err = serde.Read(stream, resp)
	if err != nil {
		stream.Reset() //nolint:errcheck
		return nil, err
	}
	err = stream.CloseRead()
	if err != nil {
		log.Warn(err)
	}
	return resp.Proofs, stream.Close()
}
