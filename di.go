package main

import (
    "flag"
    "fmt"
    "time"

    "github.com/NetSys/di/config"
    "github.com/NetSys/di/cluster"
    "github.com/op/go-logging"
)

var log = logging.MustGetLogger("main")

func main() {
    flag.Usage = func() {
        flag.PrintDefaults()
    }

    var config_path = flag.String("c", "config.json", "path to config file")
    flag.Parse()

    log.Info("Starting")
    config_chan := config.WatchConfig(*config_path)
    config := <-config_chan
    aws := cluster.New(cluster.AWS, config)
    aws.UpdateConfig(config)

    old_status := aws.GetStatus()
    fmt.Println(old_status)

    timeout := time.Tick(10 * time.Second)
    for {
        select {
            case config = <-config_chan:
                aws.UpdateConfig(config)

            case <-timeout:
                status := aws.GetStatus()
                if status != old_status {
                    old_status = status
                    fmt.Println(status)
                }
        }
    }
}
