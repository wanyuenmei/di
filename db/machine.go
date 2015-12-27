package db

import (
	"fmt"
	"sort"
	"strings"
)

// Machine represents a physical or virtual machine operated by a cloud provider on which
// containers may be run.
type Machine struct {
	/* Populated by the Database at creation time. */
	table *table
	ID    int //Database ID

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
	table := db[MachineTable]
	result := Machine{table: table, ID: table.nextID()}
	result.table.insert(result, result.ID)
	return result
}

// Write the contents of 'm' to its database.
func (m Machine) Write() {
	m.table.write(m, m.ID)
}

// Remove 'm' from its database.
func (m Machine) Remove() {
	m.table.remove(m.ID)
}

// SelectFromMachine gets all machines in the database thatsatisfy the 'check'.
func (db Database) SelectFromMachine(check func(Machine) bool) []Machine {
	result := []Machine{}
	for _, row := range db[MachineTable].rows {
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

func (m Machine) equal(r row) bool {
	b := r.(Machine)
	return m.ID == b.ID && m.ClusterID == b.ClusterID && m.Role == b.Role &&
		m.CloudID == b.CloudID && m.PublicIP == b.PublicIP &&
		m.PrivateIP == b.PrivateIP
}

// SortMachinesByID sorts 'machines' by their database IDs.
func SortMachinesByID(machines []Machine) {
	sort.Stable(machineByID(machines))
}

type machineByID []Machine

func (machines machineByID) Len() int {
	return len(machines)
}

func (machines machineByID) Swap(i, j int) {
	machines[i], machines[j] = machines[j], machines[i]
}

func (machines machineByID) Less(i, j int) bool {
	return machines[i].ID < machines[j].ID
}
