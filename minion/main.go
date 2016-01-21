//go:generate protoc ./pb/pb.proto --go_out=plugins=grpc:.
package main

import (
	"github.com/NetSys/di/db"
	"github.com/NetSys/di/minion/consensus"
	"github.com/NetSys/di/minion/docker"
	"github.com/NetSys/di/minion/scheduler"
	"github.com/NetSys/di/minion/supervisor"
	"github.com/op/go-logging"
)

var log = logging.MustGetLogger("main")

func main() {
	log.Info("Minion Start")

	conn := db.New()
	go supervisor.Run(conn, docker.New("unix:///var/run/docker.sock"))
	go scheduler.Run(conn)
	go consensus.Run(conn)
	minionServerRun(conn)
}
