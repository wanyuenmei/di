package db

import (
	"fmt"
	"strings"
)

func (conn Conn) runLogger() {
	for _, t := range allTables {
		t := t
		go func() {
			for range conn.Trigger(t).C {
				conn.logTable(t)
			}
		}()
	}
}

func (conn Conn) logTable(t TableType) {
	var strs []string
	conn.Transact(func(view Database) error {
		for _, r := range view.tables[t].rows {
			strs = append(strs, fmt.Sprintf("\t%s\n", r))
		}
		return nil
	})

	log.Info("%s", fmt.Sprintf("%s:\n%s", t, strings.Join(strs, "")))
}
