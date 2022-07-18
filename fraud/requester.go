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
	p peer.ID,
	proofTypes []int32,
) ([]*pb.RespondedProof, error) {
	msg := &pb.FraudMessage{RequestedProofType: proofTypes}
	stream, err := host.NewStream(ctx, p, fraudProtocolID)
	if err != nil {
		return nil, err
	}

	_, err = serde.Write(stream, msg)
	if err != nil {
		stream.Reset() //nolint:errcheck
		return nil, err
	}
	_, err = serde.Read(stream, msg)
	if err != nil {
		stream.Reset() //nolint:errcheck
		return nil, err
	}

	return msg.RespondedProofs, stream.Close()
}
