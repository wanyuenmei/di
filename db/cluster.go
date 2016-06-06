package db

import (
	"fmt"

	"github.com/NetSys/quilt/util"
)

// A Cluster is a group of Machines which can operate containers.
type Cluster struct {
	ID int

	Namespace string // Cloud Provider Namespace
	Spec      string

	/* XXX: These belong in a separate administration table of some sort. */
	ACLs []string
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

func (c Cluster) equal(r row) bool {
	other := r.(Cluster)
	return c.ID == other.ID &&
		c.Namespace == other.Namespace &&
		c.Spec == other.Spec &&
		util.StrSliceEqual(c.ACLs, other.ACLs)
}

func (c Cluster) getID() int {
	return c.ID
}

func (c Cluster) tt() TableType {
	return ClusterTable
}

func (c Cluster) String() string {
	return fmt.Sprintf("Cluster-%d{%s, ACL: %s}",
		c.ID, c.Namespace, c.ACLs)
}

func (c Cluster) less(r row) bool {
	return c.ID < r.(Cluster).ID
}
