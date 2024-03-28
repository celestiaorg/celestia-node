package eds

type FileMode uint8

const (
	EDSMode FileMode = iota
	ODSMode
)

type FileVersion uint8

const (
	FileV0 FileVersion = iota
)

type FileCompression uint8

const (
	NoCompression FileCompression = iota
)
