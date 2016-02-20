package db

import (
	"sort"
	"strings"

	log "github.com/Sirupsen/logrus"
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
		var rows []row
		for _, v := range view.tables[t].rows {
			rows = append(rows, v)
		}

		sort.Sort(rowSlice(rows))
		for _, r := range rows {
			strs = append(strs, r.String())
		}
		return nil
	})

	log.Infof("%s:\n\t%s", t, strings.Join(strs, "\n\t"))
}
