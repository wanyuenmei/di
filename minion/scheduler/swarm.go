package scheduler

import (
	"fmt"
	"strings"
	"sync"

	"github.com/NetSys/quilt/db"
	"github.com/NetSys/quilt/minion/docker"
	"github.com/NetSys/quilt/stitch"

	log "github.com/Sirupsen/logrus"
)

type swarm struct {
	dk docker.Client
}

func newSwarm(dk docker.Client) scheduler {
	return swarm{dk}
}

func (s swarm) list() ([]docker.Container, error) {
	return s.dk.List(map[string][]string{"label": {docker.SchedulerLabelPair}})
}

func (s swarm) boot(dbcs []db.Container, placements []db.Placement,
	connections []db.Connection) {
	numWorkers := 100
	if len(dbcs) < numWorkers {
		numWorkers = len(dbcs)
	}

	var wg sync.WaitGroup
	wg.Add(numWorkers)
	bootChn := make(chan db.Container, len(dbcs))
	logChn := make(chan string, 1)

	bootFunc := func() {
		for dbc := range bootChn {
			labels := makeLabels(dbc, connections)
			env := makeEnv(dbc, placements)
			err := s.dk.Run(docker.RunOptions{
				Image:  dbc.Image,
				Args:   dbc.Command,
				Env:    env,
				Labels: labels,
			})
			if err != nil {
				msg := fmt.Sprintf("Failed to start container %s: %s",
					dbc.Image, err)
				select {
				case logChn <- msg:
				default:
				}
			} else {
				log.Infof("Started container: %s %s", dbc.Image,
					strings.Join(dbc.Command, " "))
			}
		}
		wg.Done()
	}
	for i := 0; i < numWorkers; i++ {
		go bootFunc()
	}

	for _, dbc := range dbcs {
		bootChn <- dbc
	}

	close(bootChn)
	wg.Wait()

	select {
	case msg := <-logChn:
		log.Warning(msg)
	default:
	}
}

func makeLabels(dbc db.Container, connections []db.Connection) map[string]string {
	labels := map[string]string{
		docker.SchedulerLabelKey: docker.SchedulerLabelValue,
	}

	for _, lb := range dbc.Labels {
		// Add DSL labels
		labels[docker.UserLabel(lb)] = docker.LabelTrueValue

		// Add labels for doing placement based on ports
		for _, conn := range connections {
			if conn.From == stitch.PublicInternetLabel && conn.To == lb {
				for p := conn.MinPort; p <= conn.MaxPort; p++ {
					labels[docker.PortLabel(p)] = docker.LabelTrueValue
				}
			}
		}
	}

	return labels
}

func makeEnv(dbc db.Container, placements []db.Placement) map[string]struct{} {
	env := make(map[string]struct{})

	// Make affinity environment
	for _, placement := range placements {
		if placement.Applies(dbc) {
			env[placement.Rule.AffinityStr()] = struct{}{}
		}
	}

	// Add environment variables from the DSL
	for key, value := range dbc.Env {
		envStr := fmt.Sprintf("%s=%s", key, value)
		env[envStr] = struct{}{}
	}

	return env
}

func (s swarm) terminate(ids []string) {
	numWorkers := 100
	if len(ids) < numWorkers {
		numWorkers = len(ids)
	}

	var wg sync.WaitGroup
	wg.Add(numWorkers)
	terminateChn := make(chan string, len(ids))

	terminateFunc := func() {
		for id := range terminateChn {
			err := s.dk.RemoveID(id)
			if err != nil {
				log.WithError(err).Warn("Failed to stop container.")
			}
		}
		wg.Done()
	}
	for i := 0; i < numWorkers; i++ {
		go terminateFunc()
	}

	for _, id := range ids {
		terminateChn <- id
	}

	close(terminateChn)
	wg.Wait()
}
