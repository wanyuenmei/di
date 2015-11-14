package main

import (
    "flag"
    "fmt"
    "time"
    "io/ioutil"
    l_mod "log"

    "github.com/NetSys/di/config"
    "github.com/NetSys/di/cluster"
    "github.com/op/go-logging"
    "google.golang.org/grpc/grpclog"
)

var log = logging.MustGetLogger("main")

func main() {
    flag.Usage = func() {
        flag.PrintDefaults()
    }

    var config_path = flag.String("c", "config.json", "path to config file")
    flag.Parse()

    /* XXX: GRPC spews a lot of uselss log message so we tell to eat its logs.
    * Once we have more sophistcated logging support, we should enable the log
    * messages when in debug mode. */
    grpclog.SetLogger(l_mod.New(ioutil.Discard, "", 0))

    log.Info("Starting")
    config_chan := config.WatchConfig(*config_path)
    config := <-config_chan
    aws := cluster.New(cluster.AWS, config)
    aws.UpdateConfig(config)

    old_status := cluster.GetStatus(aws)
    fmt.Println(old_status)

    timeout := time.Tick(10 * time.Second)
    for {
        select {
            case config = <-config_chan:
                aws.UpdateConfig(config)

            case <-timeout:
                status := cluster.GetStatus(aws)
                if status != old_status {
                    old_status = status
                    fmt.Println(status)
                }
        }
    }
}
