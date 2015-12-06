package main

import (
	. "github.com/NetSys/di/minion/proto"
	"github.com/NetSys/di/minion/supervisor"
	"github.com/op/go-logging"
)

var log = logging.MustGetLogger("main")

func main() {
	log.Info("Minion Start")

	sv := supervisor.New()
	for cfg := range NewConfigChannel() {
		log.Info("Received Configuration: %s", cfg)
		sv.Configure(cfg)
	}
}
