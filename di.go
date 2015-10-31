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

    var the_config *cluster.Config
    config_chan := config.WatchConfig(config_path) 

    /* XXX: There's research in this somewhere.  How do we validate inputs into
    * the policy?  What do we do with a policy that's wrong?  Also below, we
    * want someone to be able to say "limit the number of instances for cost
    * reasons" ... look at what's going on in amp for example.  100k in a month
    * is crazy. */

    /* Block until the initial config has been parsed. */
    the_config = <-config_chan
    if the_config == nil {
        log.Fatal("Failed to read ", *config_path)
    }
    aws := cluster.New(cluster.AWS, the_config.Region)
    aws.UpdateConfig(*the_config)

    log.Info("Starting")

    for {
        select {
            case the_config = <-config_chan:
                aws.UpdateConfig(*the_config)
        }
        fmt.Println(aws.GetStatus())
        time.Sleep(10 * time.Second)
    }
}
