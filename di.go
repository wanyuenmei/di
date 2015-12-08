package main

import (
	"flag"
	"io/ioutil"
	l_mod "log"

	"github.com/NetSys/di/cluster"
	"github.com/NetSys/di/config"

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

	config.Init(*config_path)
	cluster.Run(cluster.AWS, config.Watch())
}
