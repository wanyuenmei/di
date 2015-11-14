package main

import (
    "fmt"
    "time"

    "github.com/NetSys/di/minion/proto"
)

func main() {
    log.Info("Minion Start")

    var cfg proto.MinionConfig
    for {
        cfgChan, err := NewConfigChannel()
        if err == nil {
            log.Info("Waiting for config from the master")
            cfg = <-cfgChan
            break
        }
        log.Warning("Failed to create new config channel")
        time.Sleep(10 * time.Second)
    }

    log.Info("Received Configuration: %s", cfg)
    if cfg.Role == proto.MinionConfig_MASTER {
        err := BootMaster(cfg.EtcdToken, cfg.PrivateIP)
        if err != nil {
            panic(err) /* XXX: Handle this properly. */
        }
    } else if cfg.Role == proto.MinionConfig_WORKER {
        err := BootWorker(cfg.EtcdToken)
        if err != nil {
            panic(err)
        }
    } else {
        panic(fmt.Sprintf("Unknown minion rule: %s", cfg.Role))
    }

    for ;; {
        time.Sleep(5 *time.Second)
    }
}
