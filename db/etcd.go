package db

import (
	"fmt"
	"strings"
)

type Etcd struct {
	ID int

	EtcdIPs []string

	Leader   bool   // True if this Minion is the leader.
	LeaderIP string //IP address of the current leader, or ""
}

func (e Etcd) tt() TableType {
	return EtcdTable
}

func (e Etcd) String() string {
	tags := []string{}

	if e.Leader {
		tags = append(tags, "Leader")
	}

	if e.LeaderIP != "" {
		tags = append(tags, "LeaderIP="+e.LeaderIP)
	}

	if len(e.EtcdIPs) != 0 {
		tags = append(tags, "EtcdIPs="+strings.Join(e.EtcdIPs, ", "))
	}

	return fmt.Sprintf("Etcd-%d{%s}", e.ID, strings.Join(tags, ", "))
}

func (db Database) InsertEtcd() Etcd {
	result := Etcd{ID: db.nextID()}
	db.insert(result)
	return result
}

func (db Database) SelectFromEtcd(check func(Etcd) bool) []Etcd {
	result := []Etcd{}
	for _, row := range db.tables[EtcdTable].rows {
		if check == nil || check(row.(Etcd)) {
			result = append(result, row.(Etcd))
		}
	}
	return result
}

func (conn Conn) SelectFromEtcd(check func(Etcd) bool) []Etcd {
	var etcdRows []Etcd
	conn.Transact(func(view Database) error {
		etcdRows = view.SelectFromEtcd(check)
		return nil
	})
	return etcdRows
}

func (e Etcd) id() int {
	return e.ID
}

func (e Etcd) less(r row) bool {
	return e.ID < r.(Minion).ID
}
