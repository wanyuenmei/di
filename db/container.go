package db

import (
	"fmt"
	"sort"
	"strings"
)

// A Container row is created for each container specified by the policy.  Each row will
// eventually be instantiated within its corresponding cluster. */
type Container struct {
	ID int

	ClusterID int
	SchedID   string
	Image     string
	Labels    []string
	IP        string
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

func (c Container) id() int {
	return c.ID
}

func (c Container) tt() TableType {
	return ContainerTable
}

func (c Container) String() string {
	var tags []string

	if c.ClusterID != 0 {
		tags = append(tags, fmt.Sprintf("Cluster-%d", c.ClusterID))
	}

	if c.SchedID != "" {
		tags = append(tags, fmt.Sprintf("SchedID: %s", c.SchedID))
	}

	if len(c.Labels) > 0 {
		tags = append(tags, fmt.Sprintf("Labels: %s", c.Labels))
	}

	if c.IP != "" {
		tags = append(tags, c.IP)
	}

	return fmt.Sprintf("Container-%d{%s}", c.ID, strings.Join(tags, ", "))
}

// SortContainersByID sorts 'containers' by their database IDs.
func SortContainersByID(containers []Container) {
	sort.Stable(containerByID(containers))
}

type containerByID []Container

func (containers containerByID) Len() int {
	return len(containers)
}

func (containers containerByID) Swap(i, j int) {
	containers[i], containers[j] = containers[j], containers[i]
}

func (containers containerByID) Less(i, j int) bool {
	return containers[i].ID < containers[j].ID
}
