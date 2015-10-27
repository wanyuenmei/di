package main

import (
    "flag"
    "fmt"
    "io/ioutil"
    "os"
    "strconv"
    "time"

    "github.com/NetSys/di/cluster"
    "github.com/op/go-logging"
)

var log = logging.MustGetLogger("main")

func main() {
    flag.Usage = func() {
        fmt.Fprintf(os.Stderr, "%s: count\n", os.Args[0])
        flag.PrintDefaults()
    }

    flag.Parse()
    args := flag.Args()

    if len(args) < 1 {
        panic("Number of requested instances requred.")
    }

    /* XXX: There's research in this somewhere.  How do we validate inputs into
    * the policy?  What do we do with a policy that's wrong?  Also below, we
    * want someone to be able to say "limit the number of instances for cost
    * reasons" ... look at what's going on in amp for example.  100k in a month
    * is crazy. */
    count, err := strconv.ParseInt(args[0], 10, 32)
    if err != nil {
        panic("Could not parse count.")
    }

    if count < 0 || count > 10 {
        panic("Invalid Count")
    }

    log.Info("Starting")

    cloud_config, err := ioutil.ReadFile("cloud-config.yaml")
    if err != nil {
        panic(err)
    }

    aws := cluster.New(cluster.AWS, "us-west-2")

    aws.UpdateConfig(cluster.Config {
        InstanceCount: int(count),
        CloudConfig: cloud_config,
    })

    for {
        fmt.Println(aws.GetStatus())
        time.Sleep(10 * time.Second)
    }
}
