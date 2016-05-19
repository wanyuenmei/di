package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	l_mod "log"
	"os"
	"strings"
	"text/scanner"
	"time"

	"github.com/NetSys/quilt/cluster"
	"github.com/NetSys/quilt/db"
	"github.com/NetSys/quilt/dsl"
	"github.com/NetSys/quilt/engine"
	"github.com/NetSys/quilt/util"

	"google.golang.org/grpc/grpclog"

	log "github.com/Sirupsen/logrus"
)

func main() {
	/* XXX: GRPC spews a lot of uselss log message so we tell to eat its logs.
	 * Once we have more sophistcated logging support, we should enable the log
	 * messages when in debug mode. */
	grpclog.SetLogger(l_mod.New(ioutil.Discard, "", 0))

	log.SetFormatter(util.Formatter{})

	validLevels := "Valid logger levels are:\n" +
		"    debug, info, warn, error, fatal or panic."

	flag.Usage = func() {
		fmt.Println("Usage: quilt [-c=configpath] [-log-level=level | -l=level]")
		flag.PrintDefaults()
		fmt.Println(validLevels)
	}

	var configPath = flag.String("c", "config.spec", "path to config file")
	var logLevel = flag.String("log-level", "info", "level to set logger to")
	flag.StringVar(logLevel, "l", "info", "level to set logger to")
	flag.Parse()

	level, err := parseLogLevel(*logLevel)
	if err != nil {
		fmt.Println(err)
		flag.Usage()
		os.Exit(1)
	}
	log.SetLevel(level)

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

const quiltPath = "QUILT_PATH"

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
	pathStr, _ := os.LookupEnv(quiltPath)
	pathSlice := strings.Split(pathStr, ":")
	spec, err := dsl.New(*sc.Init(bufio.NewReader(f)), pathSlice)
	if err != nil {
		return err
	}

	return engine.UpdatePolicy(conn, spec)
}

// parseLogLevel returns the log.Level type corresponding to the given string
// (case insensitive).
// If no such matching string is found, it returns log.InfoLevel (default) and an error.
func parseLogLevel(logLevel string) (log.Level, error) {
	logLevel = strings.ToLower(logLevel)
	switch logLevel {
	case "debug":
		return log.DebugLevel, nil
	case "info":
		return log.InfoLevel, nil
	case "warn":
		return log.WarnLevel, nil
	case "error":
		return log.ErrorLevel, nil
	case "fatal":
		return log.FatalLevel, nil
	case "panic":
		return log.PanicLevel, nil
	}
	return log.InfoLevel, errors.New(fmt.Sprintf("bad log level: '%v'", logLevel))
}
