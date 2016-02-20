//go:generate protoc ./pb/pb.proto --go_out=plugins=grpc:.
package main

import (
	"time"

	"github.com/NetSys/di/db"
	"github.com/NetSys/di/minion/consensus"
	"github.com/NetSys/di/minion/docker"
	"github.com/NetSys/di/minion/elector"
	"github.com/NetSys/di/minion/network"
	"github.com/NetSys/di/minion/scheduler"
	"github.com/NetSys/di/minion/supervisor"

	log "github.com/Sirupsen/logrus"
)

func main() {
	log.Info("Minion Start")

	log.SetFormatter(&log.TextFormatter{
		ForceColors:     true,
		FullTimestamp:   true,
		TimestampFormat: time.StampMilli,
	})

	conn := db.New()
	dk := docker.New("unix:///var/run/docker.sock")
	go minionServerRun(conn)
	go supervisor.Run(conn, dk)
	go scheduler.Run(conn)

	store := consensus.NewStore()
	go elector.Run(conn, store)
	go network.Run(conn, store, dk)

	select {}
}
