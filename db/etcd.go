package db

import (
	"github.com/NetSys/quilt/util"
)

// The Etcd table contains configuration pertaining to the minion etcd cluster including
// the members and leadership information.
type Etcd struct {
	ID int

	EtcdIPs []string // The set of members in the cluster.

	Leader   bool   // True if this Minion is the leader.
	LeaderIP string // IP address of the current leader, or ""
}

func (e Etcd) equal(r row) bool {
	other := r.(Etcd)
	return e.ID == other.ID &&
		util.StrSliceEqual(e.EtcdIPs, other.EtcdIPs) &&
		e.Leader == other.Leader &&
		e.LeaderIP == other.LeaderIP
}

func (e Etcd) String() string {
	return defaultString(e)
}

func (e Etcd) getID() int {
	return e.ID
}

// InsertEtcd creates a new etcd row and inserts it into the database.
func (db Database) InsertEtcd() Etcd {
	result := Etcd{ID: db.nextID()}
	db.insert(result)
	return result
}

// SelectFromEtcd gets all Etcd rows in the database that satisfy the 'check'.
func (db Database) SelectFromEtcd(check func(Etcd) bool) []Etcd {
	result := []Etcd{}
	for _, row := range db.tables[EtcdTable].rows {
		if check == nil || check(row.(Etcd)) {
			result = append(result, row.(Etcd))
		}
	}
	return result
}

// SelectFromEtcd gets all Etcd rows in the database connection that satisfy the
// 'check'.
func (conn Conn) SelectFromEtcd(check func(Etcd) bool) []Etcd {
	var etcdRows []Etcd
	conn.Transact(func(view Database) error {
		etcdRows = view.SelectFromEtcd(check)
		return nil
	})
	return etcdRows
}

func (e Etcd) less(r row) bool {
	return e.ID < r.(Minion).ID
}
