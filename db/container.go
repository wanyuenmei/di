package db

import (
	"fmt"
	"strings"

	"github.com/NetSys/di/util"
)

// A Container row is created for each container specified by the policy.  Each row will
// eventually be instantiated within its corresponding cluster.
// Used only by the minion.
type Container struct {
	ID int

	Pid     int
	IP      string
	Mac     string
	SchedID string
	Image   string
	Command []string
	Labels  []string
	Env     map[string]string

	Placement
}

// Placement represents scheduler placement constraints.
type Placement struct {
	Exclusive map[[2]string]struct{}
}

// InsertContainer creates a new container row and inserts it into the database.
func (db Database) InsertContainer() Container {
	result := Container{ID: db.nextID()}
	db.insert(result)
	return result
}

// SelectFromContainer gets all containers in the database that satisfy 'check'.
func (db Database) SelectFromContainer(check func(Container) bool) []Container {
	var result []Container
	for _, row := range db.tables[ContainerTable].rows {
		if check == nil || check(row.(Container)) {
			result = append(result, row.(Container))
		}
	}

	return result
}

// SelectFromContainer gets all containers in the database that satisfy the 'check'.
func (conn Conn) SelectFromContainer(check func(Container) bool) []Container {
	var containers []Container
	conn.Transact(func(view Database) error {
		containers = view.SelectFromContainer(check)
		return nil
	})
	return containers
}

func placementEqual(x, y map[[2]string]struct{}) bool {
	if len(x) != len(y) {
		return false
	}
	for k := range x {
		if _, ok := y[k]; !ok {
			return false
		}
	}
	return true
}

func (c Container) equal(r row) bool {
	other := r.(Container)
	return c.ID == other.ID &&
		c.Pid == other.Pid &&
		c.IP == other.IP &&
		c.Mac == other.Mac &&
		c.SchedID == other.SchedID &&
		c.Image == other.Image &&
		util.StrSliceEqual(c.Command, other.Command) &&
		util.StrSliceEqual(c.Labels, other.Labels) &&
		util.StrStrMapEqual(c.Env, other.Env) &&
		placementEqual(c.Placement.Exclusive, other.Placement.Exclusive)
}

func (c Container) getID() int {
	return c.ID
}

func (c Container) String() string {
	cmdStr := strings.Join(append([]string{"run", c.Image}, c.Command...), " ")
	tags := []string{cmdStr}

	if c.SchedID != "" {
		id := util.ShortUUID(c.SchedID)
		tags = append(tags, fmt.Sprintf("SchedID: %s", id))
	}

	if c.Pid != 0 {
		tags = append(tags, fmt.Sprintf("Pid: %d", c.Pid))
	}

	if c.IP != "" {
		tags = append(tags, fmt.Sprintf("IP: %s", c.IP))
	}

	if c.Mac != "" {
		tags = append(tags, fmt.Sprintf("Mac: %s", c.Mac))
	}

	if len(c.Labels) > 0 {
		tags = append(tags, fmt.Sprintf("Labels: %s", c.Labels))
	}

	if len(c.Placement.Exclusive) > 0 {
		tags = append(tags, fmt.Sprintf("Placement: %s", c.Placement.Exclusive))
	}

	if len(c.Env) > 0 {
		tags = append(tags, fmt.Sprintf("Env: %s", c.Env))
	}

	return fmt.Sprintf("Container-%d{%s}", c.ID, strings.Join(tags, ", "))
}

func (c Container) less(r row) bool {
	return c.ID < r.(Container).ID
}
