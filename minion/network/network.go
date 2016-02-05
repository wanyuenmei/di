package network

import (
	"github.com/NetSys/di/db"
	"github.com/NetSys/di/minion/consensus"
)

func Run(conn db.Conn, store consensus.Store) {
	go writeLabels(conn, store)
	go readLabels(conn, store)
}
