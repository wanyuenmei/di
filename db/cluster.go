package db

import (
	"fmt"
	"sort"
)

// A Cluster is a group of Machines which can operate containers.
type Cluster struct {
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
func (db Database) InsertCluster() Cluster {
	result := Cluster{ID: db.nextID()}
	db.insert(result)
	return result
}

// SelectFromCluster gets all clusters in the database that satisfy 'check'.
func (db Database) SelectFromCluster(check func(Cluster) bool) []Cluster {
	result := []Cluster{}
	for _, row := range db.tables[ClusterTable].rows {
		if check == nil || check(row.(Cluster)) {
			result = append(result, row.(Cluster))
		}
	}

	return result
}

func (c Cluster) id() int {
	return c.ID
}

func (c Cluster) tt() TableType {
	return ClusterTable
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
