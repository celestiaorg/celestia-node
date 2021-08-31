package config

type Config struct {
	P2P P2P
}

type P2P struct {
	// ListenAddresses - Addressed to listen to on local NIC.
	ListenAddresses []string
	// AnnounceAddresses - Addresses to be announced/advertised for peers to connect to
	AnnounceAddresses []string
	// NoAnnounceAddresses - Addresses the P2P subsystem may know about, but that should not be announced/advertised,
	// as undialable from WAN
	NoAnnounceAddresses []string
}
