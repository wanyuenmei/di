package scheduler

import (
	"fmt"
	"time"

	"github.com/NetSys/quilt/db"
	"github.com/NetSys/quilt/join"
	"github.com/NetSys/quilt/minion/docker"
	"github.com/NetSys/quilt/util"

	log "github.com/Sirupsen/logrus"
)

type scheduler interface {
	list() ([]docker.Container, error)

	boot(toBoot []db.Container, placement []db.Placement, connections []db.Connection)

	terminate(ids []string)
}

// Run blocks implementing the scheduler module.
func Run(conn db.Conn) {
	var sched scheduler
	for range conn.TriggerTick(30, db.MinionTable, db.EtcdTable, db.ContainerTable,
		db.PlacementTable).C {
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

		placements := conn.SelectFromPlacement(nil)
		connections := conn.SelectFromConnection(nil)
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
			sched.boot(boot, placements, connections)
		}
	}
}

func syncDB(view db.Database, dkcsArg []docker.Container) ([]string, []db.Container) {
	score := func(left, right interface{}) int {
		dbc := left.(db.Container)
		dkc := right.(docker.Container)

		// Depending on the container, the command in the database could be
		// either The command plus it's arguments, or just it's arguments.  To
		// handle that case, we check both.
		cmd1 := dkc.Args
		cmd2 := append([]string{dkc.Path}, dkc.Args...)
		dbcCmd := dbc.Command

		for key, value := range dbc.Env {
			if dkc.Env[key] != value {
				return -1
			}
		}

		var dkcLabels []string
		for label, value := range dkc.Labels {
			if !docker.IsUserLabel(label) || value != docker.LabelTrueValue {
				continue
			}
			dkcLabels = append(dkcLabels, docker.ParseUserLabel(label))
		}

		switch {
		case dkc.Image != dbc.Image:
			return -1
		case len(dbcCmd) != 0 && !strEq(dbcCmd, cmd1) && !strEq(dbcCmd, cmd2):
			return -1
		case dkc.ID == dbc.SchedID:
			return 0
		default:
			return util.EditDistance(dbc.Labels, dkcLabels)
		}
	}
	pairs, dbcs, dkcs := join.Join(view.SelectFromContainer(nil), dkcsArg, score)

	for _, pair := range pairs {
		dbc := pair.L.(db.Container)
		dbc.SchedID = pair.R.(docker.Container).ID
		view.Commit(dbc)
	}

	var term []string
	for _, dkc := range dkcs {
		term = append(term, dkc.(docker.Container).ID)
	}

	var boot []db.Container
	for _, dbc := range dbcs {
		boot = append(boot, dbc.(db.Container))
	}

	return term, boot
}

func strEq(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}
