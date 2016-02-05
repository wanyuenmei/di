package network

import (
	"github.com/NetSys/di/db"
	"github.com/NetSys/di/minion/consensus"
	"github.com/op/go-logging"
)

var log = logging.MustGetLogger("network")

func Run(conn db.Conn, store consensus.Store) {
	go readStoreRun(conn, store)
	writeStoreRun(conn, store)
}
