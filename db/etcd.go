package db

type Etcd struct {
	ID int

	EtcdIPs []string

	Leader   bool   // True if this Minion is the leader.
	LeaderIP string //IP address of the current leader, or ""
}

func (e Etcd) String() string {
	return DefaultString(e)
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

func (e Etcd) less(r row) bool {
	return e.ID < r.(Minion).ID
}
