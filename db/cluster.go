package db

import "fmt"

// A Cluster is a group of Machines which can operate containers.
type Cluster struct {
	ID int

	Provider  Provider
	Namespace string // Cloud Provider Namespace

	/* XXX: These belong in a separate Adminstration table of some sort. */
	SSHKeys  []string
	AdminACL []string
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
	return fmt.Sprintf("Cluster-%d{%s-%s, ACl: %s",
		c.ID, c.Provider, c.Namespace, c.AdminACL)
}

func (c Cluster) less(r row) bool {
	return c.ID < r.(Cluster).ID
}
