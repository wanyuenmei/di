package db

import (
	"fmt"
	"sort"
)

// A Cluster is a group of Machines which can operate containers.
type Cluster struct {
	/* Alocated by the database. */
	db *Database
	ID int

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
func (db *Database) InsertCluster() Cluster {
	result := Cluster{db: db, ID: db.nextID()}
	db.cluster.insert(result, result.ID)
	return result
}

// Write the contents of 'c' to its database.
func (c Cluster) Write() {
	c.db.cluster.write(c, c.ID)
}

// Remove 'c' from its database.
func (c Cluster) Remove() {
	c.db.cluster.remove(c.ID)
}

func (c Cluster) equal(r row) bool {
	b := r.(Cluster)

	if c.ID != b.ID || b.Namespace != b.Namespace || c.RedCount != b.RedCount ||
		c.BlueCount != b.BlueCount || len(c.SSHKeys) != len(b.SSHKeys) ||
		len(c.AdminACL) != len(b.AdminACL) {
		return false
	}

	for i := range c.SSHKeys {
		if c.SSHKeys[i] != b.SSHKeys[i] {
			return false
		}
	}

	for i := range c.AdminACL {
		if c.AdminACL[i] != b.AdminACL[i] {
			return false
		}
	}

	return true
}

// SelectFromCluster gets all clusters in the database that satisfy 'check'.
func (db Database) SelectFromCluster(check func(Cluster) bool) []Cluster {
	result := []Cluster{}
	for _, row := range db.cluster.rows {
		if check == nil || check(row.(Cluster)) {
			result = append(result, row.(Cluster))
		}
	}

	return result
}

func (c Cluster) String() string {
	return fmt.Sprintf("Cluster-%d{%s-%s, ACl: %s, Red: %d, Blue: %d}",
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
