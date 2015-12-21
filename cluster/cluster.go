package cluster

import (
	"time"

	. "github.com/NetSys/di/config"
	"github.com/NetSys/di/util"
	"github.com/op/go-logging"
)

var log = logging.MustGetLogger("cluster")

/* A group of virtual machines within a fault domain. */
type provider interface {
	get() ([]Machine, error)

	boot(count int, cloudConfig string) error

	stop(ids []string) error
}

/* Available choices of CloudProvider. */
type CloudProvider int

const (
	AWS = iota
)

/* Create a new cluster using 'provider' to host the cluster at 'region' */
func New(cp CloudProvider, cfgChan chan Config) Table {
	cfg := <-cfgChan
	log.Info("Initialized with Config: %s", cfg)

	var cloud provider
	switch cp {
	case AWS:
		cloud = newAWS(cfg.Namespace)
	default:
		panic("Cluster request for an unknown cloud provider.")
	}

	table := NewTable()
	tick := time.Tick(10 * time.Second)
	go func() {
		for {
			runOnce(cloud, table, cfg)
			select {
			case cfg = <-cfgChan:
			case <-tick:
			}
		}
	}()
	return table
}

func runOnce(cloud provider, table Table, cfg Config) {
	machines, err := cloud.get()
	if err != nil {
		log.Warning("Failed to get machines: %s", err)
		return
	}

	foremanQueryMinions(machines)

	table.set(machines)

	diff := table.diff(cfg.MasterCount, cfg.WorkerCount)

	if diff.boot > 0 {
		log.Info("Attempt to boot %d Machines", diff.boot)
		cloudConfig := util.CloudConfig(cfg.SSHKeys)
		if err := cloud.boot(diff.boot, cloudConfig); err != nil {
			log.Info("Failed to boot machines: %s", err)
		} else {
			log.Info("Successfully booted %d Machines", diff.boot)
		}
	}

	if len(diff.terminate) > 0 {
		ids := []string{}
		for _, machine := range diff.terminate {
			ids = append(ids, machine.Id)
		}

		log.Info("Attempt to stop %s", diff.terminate)
		if err := cloud.stop(ids); err != nil {
			log.Info("Failed to stop machines: %s", err)
		} else {
			log.Info("Successfully stopped %d machines", len(diff.terminate))
		}

		for _, inst := range diff.terminate {
			inst.minionClose()
		}
	}

	if len(diff.minionChange) > 0 {
		log.Info("Change Minion Config %s", diff.minionChange)
		foremanWriteMinions(diff.minionChange)
	}
}
