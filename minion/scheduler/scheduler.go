package scheduler

import (
	"fmt"
	"time"

	"github.com/NetSys/di/db"
	"github.com/NetSys/di/minion/docker"

	log "github.com/Sirupsen/logrus"
)

type scheduler interface {
	list() ([]docker.Container, error)

	boot(toBoot []db.Container)

	terminate(ids []string)
}

func Run(conn db.Conn) {
	var sched scheduler
	for range conn.TriggerTick(30, db.MinionTable, db.EtcdTable, db.ContainerTable).C {
		minions := conn.SelectFromMinion(nil)
		etcdRows := conn.SelectFromEtcd(nil)
		if len(minions) != 1 || len(etcdRows) != 1 || minions[0].Role != db.Master ||
			minions[0].PrivateIP == "" || !etcdRows[0].Leader {
			sched = nil
			continue
		}

		if sched == nil {
			ip := minions[0].PrivateIP
			sched = newSwarm(docker.New(fmt.Sprintf("tcp://%s:2377", ip)))
			time.Sleep(60 * time.Second)
		}

		// Each time we run through this loop, we may boot or terminate
		// containers.  These modification should, in turn, be reflected in the
		// database themselves.  For this reason, we attempt to sync until no
		// database modifications happen (up to an arbitrary limit of three
		// tries).
		for i := 0; i < 3; i++ {
			dkc, err := sched.list()
			if err != nil {
				log.WithError(err).Warning("Failed to get containers.")
				break
			}

			var boot []db.Container
			var term []string
			conn.Transact(func(view db.Database) error {
				term, boot = syncDB(view, dkc)
				return nil
			})

			if len(term) == 0 && len(boot) == 0 {
				break
			}
			sched.terminate(term)
			sched.boot(boot)
		}
	}
}

func syncDB(view db.Database, dkcs []docker.Container) ([]string, []db.Container) {
	var unassigned []db.Container
	cmap := make(map[string]db.Container)

	for _, c := range view.SelectFromContainer(nil) {
		if c.SchedID == "" {
			unassigned = append(unassigned, c)
		} else {
			cmap[c.SchedID] = c
		}
	}

	var term []string
	for _, dkc := range dkcs {
		if dbc, ok := cmap[dkc.ID]; ok {
			if dbc.Image == dkc.Image {
				writeContainer(view, dbc, dkc.ID)
			} else {
				writeContainer(view, dbc, "")
				term = append(term, dkc.ID)
			}
		} else if len(unassigned) > 0 {
			writeContainer(view, unassigned[0], dkc.ID)
			unassigned = unassigned[1:]
		} else {
			term = append(term, dkc.ID)
		}
	}

	return term, unassigned
}

func writeContainer(view db.Database, dbc db.Container, id string) {
	dbc.SchedID = id
	view.Commit(dbc)
}
