package container

import (
	"time"

	"github.com/NetSys/di/cluster"
	"github.com/NetSys/di/config"
	"github.com/NetSys/di/minion/pb"
	"github.com/op/go-logging"

	"golang.org/x/net/context"
)

var log = logging.MustGetLogger("container")

type controller interface {
	getContainers() map[string][]Container

	bootContainers(name string, toBoot int)

	terminateContainers(name string, toTerm []Container)
}

type ContainerController int

const (
	KUBERNETES = iota
)

/* This bit runs on the global controller. It just waits for changes to the
 * spec and ships them to the minions. */
func WatchConfig(clst cluster.Table, configChan chan config.Config) {
	for cfg := range configChan {
		/* Wait until the cluster catches up to the new config. */
		instances := make(cluster.MachineSet, 0)
		for len(instances) < cfg.MasterCount+cfg.WorkerCount {
			instances = clst.Get()
			time.Sleep(10 * time.Second)
		}

		containers := pb.ContainerConfig{
			map[string]int32{
				"red":  int32(cfg.RedCount),
				"blue": int32(cfg.BlueCount),
			},
		}

		for _, inst := range instances {
			if inst.Role != cluster.MASTER {
				continue
			}
			c := inst.MinionClient()
			if c == nil {
				continue
			}
			log.Info("Setting new container config on %s", inst)
			ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
			resp, err := c.SetContainerConfig(ctx, &containers)
			if err != nil {
				log.Info("Failed to update container config: %s", err)
			}
			if resp != nil && !resp.Success {
				log.Info("Failed to update container config: %s", resp.Error)
			}
		}
	}
}

/* The following functions run on the minion. They wait for new specs and
 * ship them to the container table for diffing. The resulting diffs are
 * then handed to the container controller (e.g. kubernetes) where the
 * appropriate booting/destruction of containers happens. */
func Run(cc ContainerController, containers chan map[string]int32) {
	cfg := <-containers
	var ctl controller
	switch cc {
	case KUBERNETES:
		ctl = NewKubectl()
	default:
		panic("Unknown container controller.")
	}

	table := NewTable()

	tick := time.Tick(10 * time.Second)
	for {
		runOnce(ctl, cfg, table)
		select {
		case cfg = <-containers:
		case <-tick:
		}
	}
}

func runOnce(ctl controller, cfg map[string]int32, table Table) {
	table.set(ctl.getContainers())
	diff := table.diff(cfg)
	for k, v := range diff.boot {
		ctl.bootContainers(k, int(v))
	}
	for k, v := range diff.terminate {
		ctl.terminateContainers(k, v)
	}
}
