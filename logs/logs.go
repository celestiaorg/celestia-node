package logs

import logging "github.com/ipfs/go-log/v2"

func SetAllLoggers(level logging.LogLevel) {
	logging.SetAllLoggers(level)
	_ = logging.SetLogLevel("engine", "FATAL")
	_ = logging.SetLogLevel("blockservice", "WARN")
	_ = logging.SetLogLevel("bs:sess", "WARN")
	_ = logging.SetLogLevel("addrutil", "INFO")
	_ = logging.SetLogLevel("dht", "ERROR")
	_ = logging.SetLogLevel("swarm2", "WARN")
	_ = logging.SetLogLevel("bitswap", "WARN")
	_ = logging.SetLogLevel("connmgr", "WARN")
	_ = logging.SetLogLevel("nat", "INFO")
	_ = logging.SetLogLevel("dht/RtRefreshManager", "FATAL")
	_ = logging.SetLogLevel("bitswap_network", "ERROR")
	_ = logging.SetLogLevel("badger", "INFO")
	_ = logging.SetLogLevel("basichost", "INFO")
	_ = logging.SetLogLevel("pubsub", "WARN")
	_ = logging.SetLogLevel("net/identify", "ERROR")
	_ = logging.SetLogLevel("shrex/nd", "WARN")
	_ = logging.SetLogLevel("shrex/eds", "WARN")
	_ = logging.SetLogLevel("fx", "FATAL")
}

func SetDebugLogging() {
	SetAllLoggers(logging.LevelDebug)
}
