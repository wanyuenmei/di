package main

import (
	"time"

	. "github.com/NetSys/di/minion/proto"
	"github.com/op/go-logging"
)

var log = logging.MustGetLogger("main")

func main() {
	log.Info("Minion Start")

	sv, err := NewSupervisor()
	if err != nil {
		panic(err) /* XXX: Do something reasonable. */
	}

	var cfg MinionConfig
	for {
		cfgChan, err := NewConfigChannel()
		if err == nil {
			log.Info("Waiting for config from the master")
			cfg = <-cfgChan
			break
		}
		log.Warning("Failed to create new config channel: %s", err)
		time.Sleep(10 * time.Second)
	}

	log.Info("Received Configuration: %s", cfg)
	if err := sv.Configure(cfg); err != nil {
		panic(err) /* XXX: Handle this properly. */
	}

	select {}
}
