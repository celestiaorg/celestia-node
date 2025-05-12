package logs

import (
	logging "github.com/ipfs/go-log/v2"
)

func SetAllLoggers(level logging.LogLevel) {
	logging.SetAllLoggers(level)
	_ = logging.SetLogLevel("addrutil", "INFO")
	_ = logging.SetLogLevel("dht", "ERROR")
	_ = logging.SetLogLevel("swarm2", "WARN")
	_ = logging.SetLogLevel("bitswap", "WARN")
	_ = logging.SetLogLevel("bitswap/client", "WARN")
	_ = logging.SetLogLevel("bitswap/client/msgq", "WARN")
	_ = logging.SetLogLevel("bitswap/client/peermgr", "WARN")
	_ = logging.SetLogLevel("bitswap/client/provqrymgr", "WARN")
	_ = logging.SetLogLevel("bitswap/client/getter", "WARN")
	_ = logging.SetLogLevel("bitswap/client/sesspeermgr", "WARN")
	_ = logging.SetLogLevel("bitswap/network", "WARN")
	_ = logging.SetLogLevel("bitswap/session", "WARN")
	_ = logging.SetLogLevel("bitswap/server", "WARN")
	_ = logging.SetLogLevel("bitswap/server/decision", "INFO")
	_ = logging.SetLogLevel("connmgr", "WARN")
	_ = logging.SetLogLevel("nat", "INFO")
	_ = logging.SetLogLevel("dht/RtRefreshManager", "FATAL")
	_ = logging.SetLogLevel("badger", "INFO")
	_ = logging.SetLogLevel("basichost", "INFO")
	_ = logging.SetLogLevel("pubsub", "WARN")
	_ = logging.SetLogLevel("net/identify", "ERROR")
	_ = logging.SetLogLevel("shrex", "WARN")
	_ = logging.SetLogLevel("fx", "FATAL")
}

func SetDebugLogging() {
	SetAllLoggers(logging.LevelDebug)
}
