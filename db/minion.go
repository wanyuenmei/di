package db

import (
	"fmt"
	"strings"
)

// The Minion table is instantiated on the minions with one row.  That row contains the
// configuration that minion needs to operate, including it's ID, Role, IP address, and
// EtcdToken
type Minion struct {
	table *table
	ID    int

	MinionID  string
	Role      Role
	PrivateIP string
	EtcdToken string

	Leader   bool   // True if this Minion is the leader.
	LeaderIP string //IP address of the current leader, or ""
}

// InsertMinion creates a new Minion and inserts it into 'db'.
func (db Database) InsertMinion() Minion {
	table := db[MinionTable]
	result := Minion{table: table, ID: table.nextID()}
	result.table.insert(result, result.ID)
	return result
}

// SelectFromMinion gets all minions in the database that satisfy the 'check'.
func (db Database) SelectFromMinion(check func(Minion) bool) []Minion {
	result := []Minion{}
	for _, row := range db[MinionTable].rows {
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

// Write the contents of 'm' to its database.
func (m Minion) Write() {
	m.table.write(m, m.ID)
}

// Remove 'm' from its database.
func (m Minion) Remove() {
	m.table.remove(m.ID)
}

func (m Minion) String() string {
	tags := []string{m.Role.String()}

	if m.MinionID != "" {
		tags = append(tags, m.MinionID)
	}

	if m.Leader {
		tags = append(tags, "Leader")
	}

	if m.LeaderIP != "" {
		tags = append(tags, "LeaderIP="+m.LeaderIP)
	}

	if m.PrivateIP != "" {
		tags = append(tags, m.PrivateIP)
	}

	if m.EtcdToken != "" {
		tags = append(tags, m.EtcdToken)
	}

	return fmt.Sprintf("Minion-%d{%s}", m.ID, strings.Join(tags, ", "))
}

func (m Minion) equal(r row) bool {
	b := r.(Minion)
	return m.ID == b.ID && m.MinionID == b.MinionID && m.Role == b.Role &&
		m.PrivateIP == b.PrivateIP && m.EtcdToken == b.EtcdToken &&
		m.Leader == b.Leader && m.LeaderIP == b.LeaderIP
}
