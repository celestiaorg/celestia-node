package share

import (
	"fmt"
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

	return Module(mod), nil
}
