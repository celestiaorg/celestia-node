package nodebuilder

import "github.com/celestiaorg/celestia-node/nodebuilder/modname"

// Module name constants to avoid duplication across the codebase
// See: https://github.com/celestiaorg/celestia-node/issues/1176
const (
	ModuleNameFraud      = modname.Fraud
	ModuleNameDAS        = modname.DAS
	ModuleNameHeader     = modname.Header
	ModuleNameState      = modname.State
	ModuleNameShare      = modname.Share
	ModuleNameP2P        = modname.P2P
	ModuleNameNode       = modname.Node
	ModuleNameBlob       = modname.Blob
	ModuleNameDA         = modname.DA
	ModuleNameBlobstream = modname.Blobstream
)
