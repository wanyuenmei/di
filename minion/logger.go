package main

import (
	"fmt"

	"github.com/NetSys/di/db"
)

func logDB(conn db.Conn) {
	go func() {
		for range conn.Trigger(db.MinionTable).C {
			logMinion(conn)
		}
	}()

	go func() {
		for range conn.Trigger(db.ContainerTable).C {
			logContainer(conn)
		}
	}()
}

func logMinion(conn db.Conn) {
	var minions []db.Minion
	conn.Transact(func(view db.Database) error {
		minions = view.SelectFromMinion(nil)
		return nil
	})

	out := "Minions:\n"
	for _, clst := range minions {
		out += fmt.Sprintf("\t%s\n", clst.String())
	}
	log.Info(out)
}

func logContainer(conn db.Conn) {
	var minions []db.Container
	conn.Transact(func(view db.Database) error {
		minions = view.SelectFromContainer(nil)
		return nil
	})

	db.SortContainersByID(minions)

	out := "Containers:\n"
	for _, clst := range minions {
		out += fmt.Sprintf("\t%s\n", clst.String())
	}
	log.Info(out)
}
