package db

import (
	"fmt"
	"sort"
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
		var rows []row
		for _, v := range view.tables[t].rows {
			rows = append(rows, v)
		}

		sort.Sort(rowSlice(rows))
		for _, r := range rows {
			strs = append(strs, fmt.Sprintf("\t%s\n", r))
		}
		return nil
	})

	log.Info("%s", fmt.Sprintf("%s:\n%s", t, strings.Join(strs, "")))
}
