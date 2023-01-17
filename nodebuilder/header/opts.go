package header

// WithBlackBoxMetrics is a header service option that wraps the header service
// with a proxied header service that records metrics for the header service.
func WithBlackBoxMetrics(hs Module) (Module, error) {
	instrumentedHeaderServ, err := newBlackBoxInstrument(hs)
	if err != nil {
		return nil, err
	}

	return instrumentedHeaderServ, nil
}
