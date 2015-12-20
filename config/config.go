package config

import (
	"bufio"
	"fmt"
	"os"
	"reflect"
	"time"

	"github.com/op/go-logging"

	"github.com/NetSys/di/dsl"
	"github.com/NetSys/di/util"
)

type Config struct {
	Namespace string

	RedCount    int
	BlueCount   int
	WorkerCount int /* Number of worker VMs */
	MasterCount int /* Number of master VMs */

	AdminACL []string
	SSHKeys  []string
}

var log = logging.MustGetLogger("config")

var cfgChan chan chan Config
var path string

/* Convert 'cfg' its string representation. */
func (cfg Config) String() string {
	str := fmt.Sprintf(
		"{\n\tNamespace: %s\n\tMasterCount: %d\n\tWorkerCount: %d\n}",
		cfg.Namespace, cfg.MasterCount, cfg.WorkerCount)
	return str
}

func parseConfig(path string) *Config {
	f, err := os.Open(path)
	if err != nil {
		log.Warning("Error opening config file: %s", err.Error())
	}
	defer f.Close()

	spec, err := dsl.New(bufio.NewReader(f))
	if err != nil {
		log.Warning(err.Error())
		return nil
	}

	config := Config{
		Namespace:   spec.QueryString("Namespace"),
		RedCount:    spec.QueryInt("RedCount"),
		BlueCount:   spec.QueryInt("BlueCount"),
		WorkerCount: spec.QueryInt("WorkerCount"),
		MasterCount: spec.QueryInt("MasterCount"),
		AdminACL:    spec.QueryStrSlice("AdminACL"),
		SSHKeys:     spec.QueryStrSlice("SSHKeys"),
	}

	if config.Namespace == "" {
		log.Warning("Must configure a Namespace")
		return nil
	}

	for i, acl := range config.AdminACL {
		if acl == "local" {
			ip, err := util.MyIp()
			if err != nil {
				log.Warning("Failed to get local IP address. Skipping ACL: %s",
					err)
			} else {
				config.AdminACL[i] = ip + "/32"
			}
		}
	}

	return &config
}

func watchConfigRun() {
	var old_config *Config
	var chns [](chan Config)

	old_config = nil
	tick := time.Tick(2 * time.Second)
	for {
		new_config := parseConfig(path)
		if new_config != nil && (old_config == nil ||
			!reflect.DeepEqual(*old_config, *new_config)) {
			old_config = new_config

			for _, chn := range chns {
				chn <- *new_config
			}
		}

		select {
		case chn := <-cfgChan:
			chns = append(chns, chn)
			if old_config != nil {
				chn <- *old_config
			}
			continue
		case <-tick:
		}
	}
}

func Init(configPath string) {
	if cfgChan == nil {
		path = configPath
		cfgChan = make(chan chan Config)
		go watchConfigRun()
	}
}

func Watch() chan Config {
	chn := make(chan Config)
	cfgChan <- chn
	return chn
}
