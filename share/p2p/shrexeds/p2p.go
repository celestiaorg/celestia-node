package shrexeds

import (
	"fmt"

	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p-core/protocol"
)

const protocolPrefix = "/shrex/eds/v0.0.1/"

var log = logging.Logger("shrex/eds")

func protocolID(protocolSuffix string) protocol.ID {
	return protocol.ID(fmt.Sprintf("%s%s", protocolPrefix, protocolSuffix))
}
