package scheduler

import (
	"github.com/NetSys/di/db"
	"github.com/NetSys/di/minion/docker"
)

type swarm struct {
	dk docker.Client
}

func newSwarm(dk docker.Client) scheduler {
	return swarm{dk}
}

func (s swarm) list() ([]docker.Container, error) {
	return s.dk.List(map[string][]string{"label": {"DI=Scheduler"}})
}

func (s swarm) boot(dbcs []db.Container) error {
	for _, dbc := range dbcs {
		err := s.dk.Run(docker.RunOptions{
			Image:  dbc.Image,
			Args:   dbc.Command,
			Labels: map[string]string{"DI": "Scheduler"},
		})
		if err != nil {
			return err
		}
	}

	return nil
}

func (s swarm) terminate(ids []string) error {
	for _, id := range ids {
		err := s.dk.RemoveID(id)
		if err != nil {
			return err
		}
	}

	return nil
}
