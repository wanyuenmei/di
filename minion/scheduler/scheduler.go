package scheduler

import "github.com/NetSys/di/db"

type Container struct {
	ID    string
	IP    string
	Label string
}

type scheduler interface {
	get() ([]Container, error)

	boot(toBoot int)

	terminate(ids []string)
}

func Run(conn db.Conn) {
	var sched scheduler
	for range conn.TriggerTick(30, db.MinionTable, db.ContainerTable).C {
		minions := conn.SelectFromMinion(nil)
		if len(minions) != 1 || minions[0].Role != db.Master ||
			!minions[0].Leader {
			sched = nil
			continue
		}

		if sched == nil {
			var err error
			sched, err = NewKubectl()
			if err != nil {
				continue
			}
		}

		syncContainers(conn, sched)
	}
}

func syncContainers(conn db.Conn, sched scheduler) {
	for i := 0; i < 8; i++ {
		schedC, err := sched.get()
		if err != nil {
			log.Warning("Failed to get containers: %s", err)
			return
		}

		var nBoot int
		var term []string
		conn.Transact(func(view db.Database) error {
			term, nBoot = syncDB(view, schedC)
			return nil
		})

		sched.boot(nBoot)
		sched.terminate(term)
	}
}

func syncDB(view db.Database, schedC []Container) (term []string, nBoot int) {
	containers := view.SelectFromContainer(nil)
	var unassigned []db.Container
	cmap := make(map[string]db.Container)
	for _, c := range containers {
		if c.SchedID == "" {
			unassigned = append(unassigned, c)
		} else {
			cmap[c.SchedID] = c
		}
	}

	for _, sc := range schedC {
		if dbc, ok := cmap[sc.ID]; ok {
			/* XXX: Change the label without rebooting the container. */
			if dbc.Label == sc.Label {
				writeContainer(view, dbc, sc)
			} else {
				term = append(term, sc.ID)
			}
		} else if len(unassigned) > 0 {
			writeContainer(view, unassigned[0], sc)
			unassigned = unassigned[1:]
		} else {
			term = append(term, sc.ID)
		}
	}

	return term, len(unassigned)
}

func writeContainer(view db.Database, dbc db.Container, sc Container) {
	dbc.SchedID = sc.ID
	dbc.IP = sc.IP
	view.Commit(dbc)
}
