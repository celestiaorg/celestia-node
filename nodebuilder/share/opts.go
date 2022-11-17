package share

func WithBlackBoxMetrics(ss Module) (Module, error) {
	instrumentedShareServ, err := newBlackBoxInstrument(ss)
	if err != nil {
		return nil, err
	}

	return instrumentedShareServ, nil
}
