package scheduler

import (
	"fmt"
	"runtime/debug"

	"github.com/NetSys/di/db"
	"github.com/NetSys/di/minion/docker"
	"github.com/op/go-logging"
)

var log = logging.MustGetLogger("scheduler")

type scheduler interface {
	list() ([]docker.Container, error)

	boot(toBoot []db.Container)

	terminate(ids []string)
}

func Run(conn db.Conn) {
	var sched scheduler
	for range conn.TriggerTick(10, db.MinionTable, db.ContainerTable).C {
		minions := conn.SelectFromMinion(nil)
		if len(minions) != 1 || minions[0].Role != db.Master ||
			minions[0].PrivateIP == "" || !minions[0].Leader {
			sched = nil
			continue
		}

		if sched == nil {
			ip := minions[0].PrivateIP
			sched = newSwarm(docker.New(fmt.Sprintf("tcp://%s:2377", ip)))
		}

		// Each time we run through this loop, we may boot or terminate
		// containers.  These modification should, in turn, be reflected in the
		// database themselves.  For this reason, we attempt to sync until no
		// database modifications happen (up to an arbitrary limit of three
		// tries).
		for i := 0; i < 3; i++ {
			dkc, err := sched.list()
			if err != nil {
				log.Warning("Failed to get containers: %s", err)
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
				writeContainer(view, dbc, dkc.ID, dkc.IPs)
			} else {
				writeContainer(view, dbc, "", nil)
				term = append(term, dkc.ID)
			}
		} else if len(unassigned) > 0 {
			writeContainer(view, unassigned[0], dkc.ID, dkc.IPs)
			unassigned = unassigned[1:]
		} else {
			term = append(term, dkc.ID)
		}
	}

	return term, unassigned
}

func writeContainer(view db.Database, dbc db.Container, id string, ips []string) {
	dbc.SchedID = id
	dbc.IP = ""

	if len(ips) > 1 {
		panic("Unimplemented\n" + string(debug.Stack()))
	} else if len(ips) == 1 {
		dbc.IP = ips[0]
	}

	view.Commit(dbc)
}
