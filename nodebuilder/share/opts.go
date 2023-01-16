package share

import (
	"fmt"

	"github.com/celestiaorg/celestia-node/share"
)

func WithBlackBoxMetrics(ss Module) (Module, error) {
	mod, ok := ss.(*module)
	if !ok {
		return nil, fmt.Errorf("[WithBlackBoxMetrics]: encountered an error while type-casting share.Module to `module` ")
	}

	insShareGetter, err := newInstrument(mod.Getter)
	if err != nil {
		return nil, err
	}

	// Reassign `Availability` with the instrumented
	// availability instance here
	// once it's implemented
	mod.Getter = insShareGetter
	pmod, err := Proxy(mod, "Getter", insShareGetter)
	if err != nil {
		return nil, err
	}

	return pmod, nil
}

func Proxy(mod *module, field string, proxy any) (Module, error) {
	switch field {
	case "Getter":
		getter, ok := proxy.(share.Getter)
		if !ok {
			return nil, fmt.Errorf("trying to proxy `Getter` but provided a different type")
		}

		mod.Getter = getter
		return mod, nil

	default:
		// add logging
		return mod, nil
	}
}
