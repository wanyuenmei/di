package db

import (
	"fmt"
	"sort"
)

// A Container row is created for each container specified by the policy.  Each row will
// eventually be instantiated within its corresponding cluster. */
type Container struct {
	table *table
	ID    int

	SchedID string
	Label   string
	IP      string
}

// InsertContainer creates a new container row and inserts it into the database.
func (db Database) InsertContainer() Container {
	table := db[ContainerTable]
	result := Container{table: table, ID: table.nextID()}
	result.table.insert(result, result.ID)
	return result
}

// SelectFromContainer gets all containers in the database that satisfy 'check'.
func (db Database) SelectFromContainer(check func(Container) bool) []Container {
	var result []Container
	for _, row := range db[ContainerTable].rows {
		if check == nil || check(row.(Container)) {
			result = append(result, row.(Container))
		}
	}

	return result
}

// Write the contents of 'c' to its database.
func (c Container) Write() {
	c.table.write(c, c.ID)
}

// Remove 'c' from its database.
func (c Container) Remove() {
	c.table.remove(c.ID)
}

func (c Container) equal(r row) bool {
	b := r.(Container)
	return c.ID == b.ID && c.Label == b.Label
}

func (c Container) String() string {
	return fmt.Sprintf("Container-%d{%s}", c.ID, c.Label)
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
