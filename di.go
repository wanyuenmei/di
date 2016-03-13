package main

import (
	"bufio"
	"flag"
	"io/ioutil"
	l_mod "log"
	"text/scanner"
	"time"

	"github.com/NetSys/di/cluster"
	"github.com/NetSys/di/db"
	"github.com/NetSys/di/dsl"
	"github.com/NetSys/di/engine"
	"github.com/NetSys/di/util"

	"google.golang.org/grpc/grpclog"

	log "github.com/Sirupsen/logrus"
)

func main() {
	/* XXX: GRPC spews a lot of uselss log message so we tell to eat its logs.
	 * Once we have more sophistcated logging support, we should enable the log
	 * messages when in debug mode. */
	grpclog.SetLogger(l_mod.New(ioutil.Discard, "", 0))

	log.SetFormatter(util.Formatter{})

	flag.Usage = func() {
		flag.PrintDefaults()
	}

	var configPath = flag.String("c", "config.spec", "path to config file")
	flag.Parse()

	conn := db.New()
	go func() {
		tick := time.Tick(5 * time.Second)
		for {
			if err := updateConfig(conn, *configPath); err != nil {
				log.WithError(err).Warn(
					"Failed to update configuration.")
			}

			select {
			case <-tick:
			}
		}
	}()

	cluster.Run(conn)
}

func updateConfig(conn db.Conn, configPath string) error {
	f, err := util.Open(configPath)
	if err != nil {
		return err
	}
	defer f.Close()

	sc := scanner.Scanner{
		Position: scanner.Position{
			Filename: configPath,
		},
	}
	spec, err := dsl.New(*sc.Init(bufio.NewReader(f)))
	if err != nil {
		return err
	}

	return engine.UpdatePolicy(conn, spec)
}
