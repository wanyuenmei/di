package db

// The Minion table is instantiated on the minions with one row.  That row contains the
// configuration that minion needs to operate, including its ID, Role, and IP address
type Minion struct {
	ID int

	MinionID  string
	Role      Role
	PrivateIP string
}

// InsertMinion creates a new Minion and inserts it into 'db'.
func (db Database) InsertMinion() Minion {
	result := Minion{ID: db.nextID()}
	db.insert(result)
	return result
}

// SelectFromMinion gets all minions in the database that satisfy the 'check'.
func (db Database) SelectFromMinion(check func(Minion) bool) []Minion {
	result := []Minion{}
	for _, row := range db.tables[MinionTable].rows {
		if check == nil || check(row.(Minion)) {
			result = append(result, row.(Minion))
		}
	}
	return result
}

// SelectFromMinion gets all minions in the database that satisfy the 'check'.
func (conn Conn) SelectFromMinion(check func(Minion) bool) []Minion {
	var minions []Minion
	conn.Transact(func(view Database) error {
		minions = view.SelectFromMinion(check)
		return nil
	})
	return minions
}

func (m Minion) String() string {
	return defaultString(m)
}

func (m Minion) less(r row) bool {
	return m.ID < r.(Minion).ID
}
