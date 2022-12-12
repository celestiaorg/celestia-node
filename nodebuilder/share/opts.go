package share

import (
	"fmt"

	"github.com/celestiaorg/celestia-node/share/service"
)

func WithBlackBoxMetrics(ss Module) (Module, error) {
	mod, ok := ss.(*module)
	if !ok {
		return nil, fmt.Errorf("[WithBlackBoxMetrics]: encountered an error while type-casting share.Module to `module` ")
	}
	shareService, err := WithBlackBoxShareServiceMetrics(mod.ShareService)
	if err != nil {
		return nil, err
	}

	// Add `Availability` once implemented
	mod.ShareService = shareService

	return Module(mod), nil
}

func WithBlackBoxShareServiceMetrics(ss service.ShareService) (service.ShareService, error) {
	instrumentedShareServ, err := newBlackBoxInstrument(ss)
	if err != nil {
		return nil, err
	}

	return instrumentedShareServ, nil
}
