package share

import (
	"context"

	"github.com/celestiaorg/celestia-node/share"
)

var _ RDANodeModule = (*RDANodeAPI)(nil)

// RDANodeModule defines the RPC interface for the RDA grid system.
// Any method signature changed here needs to also be changed in the RDANodeAPI struct.
type RDANodeModule interface {
	// GetMyPosition returns this node's position in the RDA grid.
	GetMyPosition(ctx context.Context) (*share.RDAPosition, error)
	// GetStatus returns the current status of the RDA node.
	GetStatus(ctx context.Context) (*share.RDAStatus, error)
	// GetNodeInfo returns detailed information about this RDA node.
	GetNodeInfo(ctx context.Context) (*share.RDANodeInfo, error)
	// GetRowPeers returns all peers in the same row.
	GetRowPeers(ctx context.Context) (*share.RDAPeerList, error)
	// GetColPeers returns all peers in the same column.
	GetColPeers(ctx context.Context) (*share.RDAPeerList, error)
	// GetSubnetPeers returns all peers in the subnet (row + column).
	GetSubnetPeers(ctx context.Context) (*share.RDAPeerList, error)
	// GetGridDimensions returns the dimensions of the RDA grid.
	GetGridDimensions(ctx context.Context) (*share.RDAGridInfo, error)
	// GetStats returns RDA operation statistics.
	GetStats(ctx context.Context) (*share.RDAStats, error)
	// GetHealth returns health status of the RDA node.
	GetHealth(ctx context.Context) (*share.RDAHealthStatus, error)
	// PublishToSubnet publishes data to all subnet peers (row + column).
	PublishToSubnet(ctx context.Context, req *share.RDAPublishRequest) (*share.RDAPublishResponse, error)
	// PublishToRow publishes data only to row peers.
	PublishToRow(ctx context.Context, req *share.RDAPublishRequest) (*share.RDAPublishResponse, error)
	// PublishToCol publishes data only to column peers.
	PublishToCol(ctx context.Context, req *share.RDAPublishRequest) (*share.RDAPublishResponse, error)
	// RequestDataFromRow requests data from row peers.
	RequestDataFromRow(ctx context.Context, req *share.RDADataRequest) (*share.RDABatchDataResponse, error)
	// RequestDataFromCol requests data from column peers.
	RequestDataFromCol(ctx context.Context, req *share.RDADataRequest) (*share.RDABatchDataResponse, error)
	// RequestDataFromSubnet requests data from all subnet peers.
	RequestDataFromSubnet(ctx context.Context, req *share.RDADataRequest) (*share.RDABatchDataResponse, error)
	// GetSubnetMembers returns the list of members in this node's subnets.
	GetSubnetMembers(ctx context.Context) (*share.RDASubnetMembersResponse, error)
	// AnnounceToSubnet manually triggers announcement to subnets.
	AnnounceToSubnet(ctx context.Context) (*share.RDAAnnouncementResponse, error)
}

// RDANodeAPI is a wrapper around RDAModule for the JSON-RPC server.
// It follows the standard nodebuilder API pattern with Internal function fields
// that get populated by the auth.PermissionedProxy middleware.
type RDANodeAPI struct {
	Internal struct {
		GetMyPosition         func(ctx context.Context) (*share.RDAPosition, error)                                      `perm:"read"`
		GetStatus             func(ctx context.Context) (*share.RDAStatus, error)                                        `perm:"read"`
		GetNodeInfo           func(ctx context.Context) (*share.RDANodeInfo, error)                                      `perm:"read"`
		GetRowPeers           func(ctx context.Context) (*share.RDAPeerList, error)                                      `perm:"read"`
		GetColPeers           func(ctx context.Context) (*share.RDAPeerList, error)                                      `perm:"read"`
		GetSubnetPeers        func(ctx context.Context) (*share.RDAPeerList, error)                                      `perm:"read"`
		GetGridDimensions     func(ctx context.Context) (*share.RDAGridInfo, error)                                      `perm:"read"`
		GetStats              func(ctx context.Context) (*share.RDAStats, error)                                         `perm:"read"`
		GetHealth             func(ctx context.Context) (*share.RDAHealthStatus, error)                                  `perm:"read"`
		PublishToSubnet       func(ctx context.Context, req *share.RDAPublishRequest) (*share.RDAPublishResponse, error) `perm:"write"`
		PublishToRow          func(ctx context.Context, req *share.RDAPublishRequest) (*share.RDAPublishResponse, error) `perm:"write"`
		PublishToCol          func(ctx context.Context, req *share.RDAPublishRequest) (*share.RDAPublishResponse, error) `perm:"write"`
		RequestDataFromRow    func(ctx context.Context, req *share.RDADataRequest) (*share.RDABatchDataResponse, error)  `perm:"read"`
		RequestDataFromCol    func(ctx context.Context, req *share.RDADataRequest) (*share.RDABatchDataResponse, error)  `perm:"read"`
		RequestDataFromSubnet func(ctx context.Context, req *share.RDADataRequest) (*share.RDABatchDataResponse, error)  `perm:"read"`
		GetSubnetMembers      func(ctx context.Context) (*share.RDASubnetMembersResponse, error)                         `perm:"read"`
		AnnounceToSubnet      func(ctx context.Context) (*share.RDAAnnouncementResponse, error)                          `perm:"write"`
	}
}

func (api *RDANodeAPI) GetMyPosition(ctx context.Context) (*share.RDAPosition, error) {
	return api.Internal.GetMyPosition(ctx)
}

func (api *RDANodeAPI) GetStatus(ctx context.Context) (*share.RDAStatus, error) {
	return api.Internal.GetStatus(ctx)
}

func (api *RDANodeAPI) GetNodeInfo(ctx context.Context) (*share.RDANodeInfo, error) {
	return api.Internal.GetNodeInfo(ctx)
}

func (api *RDANodeAPI) GetRowPeers(ctx context.Context) (*share.RDAPeerList, error) {
	return api.Internal.GetRowPeers(ctx)
}

func (api *RDANodeAPI) GetColPeers(ctx context.Context) (*share.RDAPeerList, error) {
	return api.Internal.GetColPeers(ctx)
}

func (api *RDANodeAPI) GetSubnetPeers(ctx context.Context) (*share.RDAPeerList, error) {
	return api.Internal.GetSubnetPeers(ctx)
}

func (api *RDANodeAPI) GetGridDimensions(ctx context.Context) (*share.RDAGridInfo, error) {
	return api.Internal.GetGridDimensions(ctx)
}

func (api *RDANodeAPI) GetStats(ctx context.Context) (*share.RDAStats, error) {
	return api.Internal.GetStats(ctx)
}

func (api *RDANodeAPI) GetHealth(ctx context.Context) (*share.RDAHealthStatus, error) {
	return api.Internal.GetHealth(ctx)
}

func (api *RDANodeAPI) PublishToSubnet(ctx context.Context, req *share.RDAPublishRequest) (*share.RDAPublishResponse, error) {
	return api.Internal.PublishToSubnet(ctx, req)
}

func (api *RDANodeAPI) PublishToRow(ctx context.Context, req *share.RDAPublishRequest) (*share.RDAPublishResponse, error) {
	return api.Internal.PublishToRow(ctx, req)
}

func (api *RDANodeAPI) PublishToCol(ctx context.Context, req *share.RDAPublishRequest) (*share.RDAPublishResponse, error) {
	return api.Internal.PublishToCol(ctx, req)
}

func (api *RDANodeAPI) RequestDataFromRow(ctx context.Context, req *share.RDADataRequest) (*share.RDABatchDataResponse, error) {
	return api.Internal.RequestDataFromRow(ctx, req)
}

func (api *RDANodeAPI) RequestDataFromCol(ctx context.Context, req *share.RDADataRequest) (*share.RDABatchDataResponse, error) {
	return api.Internal.RequestDataFromCol(ctx, req)
}

func (api *RDANodeAPI) RequestDataFromSubnet(ctx context.Context, req *share.RDADataRequest) (*share.RDABatchDataResponse, error) {
	return api.Internal.RequestDataFromSubnet(ctx, req)
}

func (api *RDANodeAPI) GetSubnetMembers(ctx context.Context) (*share.RDASubnetMembersResponse, error) {
	return api.Internal.GetSubnetMembers(ctx)
}

func (api *RDANodeAPI) AnnounceToSubnet(ctx context.Context) (*share.RDAAnnouncementResponse, error) {
	return api.Internal.AnnounceToSubnet(ctx)
}
