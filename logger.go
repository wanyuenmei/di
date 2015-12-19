package main

import (
	"fmt"

	"github.com/NetSys/di/db"
)

func logDB(conn db.Conn) {
	go func() {
		for range conn.Trigger("Cluster").C {
			logCluster(conn)
		}
	}()

	go func() {
		for range conn.Trigger("Machine").C {
			logMachine(conn)
		}
	}()
}

func logCluster(conn db.Conn) {
	var clusters []db.Cluster
	conn.Transact(func(view *db.Database) error {
		clusters = view.SelectFromCluster(nil)
		return nil
	})

	db.SortClustersByID(clusters)

	out := "Clusters:\n"
	for _, clst := range clusters {
		out += fmt.Sprintf("\t%s\n", clst.String())
	}
	log.Info(out)
}

func logMachine(conn db.Conn) {
	var masters, workers []db.Machine
	conn.Transact(func(view *db.Database) error {
		masters = view.SelectFromMachine(func(m db.Machine) bool {
			return m.Role == db.Master
		})
		workers = view.SelectFromMachine(func(m db.Machine) bool {
			return m.Role == db.Worker
		})
		return nil
	})

	db.SortMachinesByID(masters)
	db.SortMachinesByID(workers)

	out := "Machines:\nMasters:\n"
	for _, m := range masters {
		out += fmt.Sprintf("\t%s\n", m.String())
	}

	out += "Workers:\n"
	for _, m := range workers {
		out += fmt.Sprintf("\t%s\n", m.String())
	}
	log.Info(out)
}
