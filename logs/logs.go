package logs

import logging "github.com/ipfs/go-log/v2"

func SetAllLoggers(level logging.LogLevel) {
	logging.SetAllLoggers(level)
	_ = logging.SetLogLevel("bs:sess", "WARN")
	_ = logging.SetLogLevel("addrutil", "INFO")
	_ = logging.SetLogLevel("dht", "ERROR")
	_ = logging.SetLogLevel("swarm2", "WARN")
	_ = logging.SetLogLevel("bitswap", "WARN")
	_ = logging.SetLogLevel("connmgr", "WARN")
	_ = logging.SetLogLevel("nat", "INFO")
	_ = logging.SetLogLevel("dht/RtRefreshManager", "FATAL")
}
