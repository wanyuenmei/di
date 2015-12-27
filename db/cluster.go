package db

import (
	"fmt"
	"sort"
)

// A Cluster is a group of Machines which can operate containers.
type Cluster struct {
	/* Alocated by the database. */
	table *table
	ID    int

	Provider  Provider
	Namespace string // Cloud Provider Namespace

	/* XXX: These belong in a separate Adminstration table of some sort. */
	SSHKeys  []string
	AdminACL []string

	/* XXX: These Belong in their own container configuration table. */
	RedCount  int
	BlueCount int
}

// InsertCluster creates a new Cluster and interts it into 'db'.
func (db Database) InsertCluster() Cluster {
	table := db[ClusterTable]
	result := Cluster{table: table, ID: table.nextID()}
	result.table.insert(result, result.ID)
	return result
}

// SelectFromCluster gets all clusters in the database that satisfy 'check'.
func (db Database) SelectFromCluster(check func(Cluster) bool) []Cluster {
	result := []Cluster{}
	for _, row := range db[ClusterTable].rows {
		if check == nil || check(row.(Cluster)) {
			result = append(result, row.(Cluster))
		}
	}

	return result
}

// Write the contents of 'c' to its database.
func (c Cluster) Write() {
	c.table.write(c, c.ID)
}

// Remove 'c' from its database.
func (c Cluster) Remove() {
	c.table.remove(c.ID)
}

func (c Cluster) equal(r row) bool {
	b := r.(Cluster)
	return c.ID == b.ID && b.Namespace == b.Namespace &&
		strSliceEqual(c.SSHKeys, b.SSHKeys) &&
		strSliceEqual(c.AdminACL, b.AdminACL) &&
		c.RedCount == b.RedCount && c.BlueCount == b.BlueCount
}

func (c Cluster) String() string {
	return fmt.Sprintf("Cluster-%d{%s-%s, ACl: %s, RedCount: %d, BlueCount %d}",
		c.ID, c.Provider, c.Namespace, c.AdminACL, c.RedCount, c.BlueCount)
}

// SortClustersByID sorts 'clusters' by their database ID.
func SortClustersByID(clusters []Cluster) {
	sort.Stable(clusterByID(clusters))
}

type clusterByID []Cluster

func (clusters clusterByID) Len() int {
	return len(clusters)
}

func (clusters clusterByID) Swap(i, j int) {
	clusters[i], clusters[j] = clusters[j], clusters[i]
}

func (clusters clusterByID) Less(i, j int) bool {
	return clusters[i].ID < clusters[j].ID
}
