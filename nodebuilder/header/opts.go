package header

func WithBlackBoxMetrics(hs Module) (Module, error) {
	instrumentedHeaderServ, err := newBlackBoxInstrument(hs)
	if err != nil {
		return nil, err
	}

	return instrumentedHeaderServ, nil
}
