package db

import (
	"sort"

	"github.com/NetSys/quilt/util"
)

// Machine represents a physical or virtual machine operated by a cloud provider on
// which containers may be run.
type Machine struct {
	ID int //Database ID

	/* Populated by the policy engine. */
	ClusterID int //Parent Cluster ID
	Role      Role
	Provider  Provider
	Region    string
	Size      string
	DiskSize  int
	SSHKeys   []string `rowStringer:"omit"`

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

// SelectFromMachine gets all machines in the database that satisfy the 'check'.
func (db Database) SelectFromMachine(check func(Machine) bool) []Machine {
	result := []Machine{}
	for _, row := range db.tables[MachineTable].rows {
		if check == nil || check(row.(Machine)) {
			result = append(result, row.(Machine))
		}
	}
	return result
}

func (m Machine) equal(r row) bool {
	other := r.(Machine)
	return m.ID == other.ID &&
		m.ClusterID == other.ClusterID &&
		m.Role == other.Role &&
		m.Provider == other.Provider &&
		m.Region == other.Region &&
		m.Size == other.Size &&
		m.DiskSize == other.DiskSize &&
		util.StrSliceEqual(m.SSHKeys, other.SSHKeys) &&
		m.CloudID == other.CloudID &&
		m.PublicIP == other.PublicIP &&
		m.PrivateIP == other.PrivateIP
}

func (m Machine) getID() int {
	return m.ID
}

func (m Machine) String() string {
	return defaultString(m)
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

// SortMachines returns a slice of machines sorted according to the default database
// sort order.
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

// MachineSlice is an alias for []Machine to allow for joins
type MachineSlice []Machine

// Get returns the value contained at the given index
func (ms MachineSlice) Get(ii int) interface{} {
	return ms[ii]
}

// Len returns the number of items in the slice
func (ms MachineSlice) Len() int {
	return len(ms)
}
