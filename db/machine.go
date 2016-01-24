package db

import (
	"fmt"
	"sort"
	"strings"
)

// Machine represents a physical or virtual machine operated by a cloud provider on which
// containers may be run.
type Machine struct {
	ID int //Database ID

	/* Populated by the policy engine. */
	ClusterID int //Parent Cluster ID
	Role      Role

	/* Populated by the cloud provider. */
	CloudID   string //Cloud Provider ID
	PublicIP  string
	PrivateIP string
}

// InsertMachine creates a new Machine and inserts it into 'db'.
func (db Database) InsertMachine() Machine {
	result := Machine{ID: db.nextID()}
	db.insert(result)
	return result
}

func (m Machine) id() int {
	return m.ID
}

// Remove 'm' from its database.
func (m Machine) tt() TableType {
	return MachineTable
}

// SelectFromMachine gets all machines in the database thatsatisfy the 'check'.
func (db Database) SelectFromMachine(check func(Machine) bool) []Machine {
	result := []Machine{}
	for _, row := range db.tables[MachineTable].rows {
		if check == nil || check(row.(Machine)) {
			result = append(result, row.(Machine))
		}
	}
	return result
}

func (m Machine) String() string {
	tags := []string{fmt.Sprintf("Cluster-%d", m.ClusterID)}

	if m.CloudID != "" {
		tags = append(tags, m.CloudID)
	}

	tags = append(tags, m.Role.String())

	if m.PublicIP != "" {
		tags = append(tags, m.PublicIP)
	}

	if m.PrivateIP != "" {
		tags = append(tags, m.PrivateIP)
	}

	return fmt.Sprintf("Machine-%d{%s}", m.ID, strings.Join(tags, ", "))
}

func (m Machine) less(arg row) bool {
	l, r := m, arg.(Machine)
	upl := l.PublicIP != "" && l.PrivateIP != ""
	upr := r.PublicIP != "" && r.PrivateIP != ""
	downl := l.PublicIP == "" && l.PrivateIP == ""
	downr := r.PublicIP == "" && r.PrivateIP == ""

	switch {
	case l.Role != r.Role:
		return l.Role > r.Role
	case upl != upr:
		return upl
	case downl != downr:
		return !downl
	case l.CloudID != r.CloudID:
		return l.CloudID < r.CloudID
	default:
		return l.ID < r.ID
	}
}

func SortMachines(machines []Machine) []Machine {
	rows := make([]row, 0, len(machines))
	for _, m := range machines {
		rows = append(rows, m)
	}

	sort.Sort(rowSlice(rows))

	machines = make([]Machine, 0, len(machines))
	for _, r := range rows {
		machines = append(machines, r.(Machine))
	}

	return machines
}
