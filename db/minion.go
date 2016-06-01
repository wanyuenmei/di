package db

import "errors"

// The Minion table is instantiated on the minions with one row.  That row contains the
// configuration that minion needs to operate, including its ID, Role, and IP address
type Minion struct {
	ID int

	Role      Role
	PrivateIP string
	Spec      string
	Self      bool

	Provider string
	Size     string
	Region   string
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

// MinionSelf returns the Minion Row corresponding to the currently running minion, or an
// error if no such row exists.
func (db Database) MinionSelf() (Minion, error) {
	minions := db.SelectFromMinion(func(m Minion) bool {
		return m.Self
	})

	if len(minions) > 1 {
		panic("multiple minions labeled Self")
	}

	if len(minions) == 0 {
		return Minion{}, errors.New("no self minion")
	}

	return minions[0], nil
}

// MinionSelf returns the Minion Row corresponding to the currently running minion, or an
// error if no such row exists.
func (conn Conn) MinionSelf() (Minion, error) {
	var m Minion
	var err error

	conn.Transact(func(view Database) error {
		m, err = view.MinionSelf()
		return nil
	})

	return m, err
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

func (m Minion) equal(r row) bool {
	return m == r.(Minion)
}

func (m Minion) getID() int {
	return m.ID
}

func (m Minion) String() string {
	return defaultString(m)
}

func (m Minion) less(r row) bool {
	return m.ID < r.(Minion).ID
}
