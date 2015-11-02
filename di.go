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
    the_config := <-config_chan
    aws := cluster.New(cluster.AWS, the_config.Region)
    aws.UpdateConfig(the_config)

    for {
        select {
            case the_config = <-config_chan:
                aws.UpdateConfig(the_config)
        }
        fmt.Println(aws.GetStatus())
        time.Sleep(10 * time.Second)
    }
}
