package engine

import (
	"fmt"

	"github.com/NetSys/di/db"
	"github.com/NetSys/di/dsl"
	"github.com/NetSys/di/util"
	"github.com/op/go-logging"
)

var log = logging.MustGetLogger("engine")
var myIP = util.MyIp

// UpdatePolicy executes transactions on 'conn' to make it reflect a new policy, 'dsl'.
func UpdatePolicy(conn db.Conn, dsl dsl.Dsl) error {
	txn := func(db db.Database) error {
		return updateTxn(db, dsl)
	}

	if err := conn.Transact(txn); err != nil {
		return err
	}

	return nil
}

func updateTxn(view db.Database, dsl dsl.Dsl) error {
	cluster, err := clusterTxn(view, dsl)
	if err != nil {
		return err
	}

	if err = machineTxn(view, dsl, cluster); err != nil {
		return err
	}

	if err = containerTxn(view, dsl, cluster); err != nil {
		return err
	}

	return nil
}

func clusterTxn(view db.Database, dsl dsl.Dsl) (int, error) {
	Namespace := dsl.QueryString("Namespace")
	if Namespace == "" {
		return 0, fmt.Errorf("Policy must specify a 'Namespace'")
	}

	provider, err := db.ParseProvider(dsl.QueryString("Provider"))
	if err != nil {
		return 0, err
	}

	var cluster db.Cluster
	clusters := view.SelectFromCluster(nil)
	switch len(clusters) {
	case 1:
		cluster = clusters[0]
	case 0:
		cluster = view.InsertCluster()
	default:
		panic("Unimplemented")
	}

	cluster.Provider = provider
	cluster.Namespace = Namespace
	cluster.AdminACL = resolveACLs(dsl.QueryStrSlice("AdminACL"))
	cluster.SSHKeys = dsl.QueryStrSlice("SSHKeys")
	view.Commit(cluster)

	return cluster.ID, nil
}

func machineTxn(view db.Database, dsl dsl.Dsl, clusterID int) error {
	masterCount := dsl.QueryInt("MasterCount")
	workerCount := dsl.QueryInt("WorkerCount")
	if masterCount == 0 || workerCount == 0 {
		masterCount = 0
		workerCount = 0
	}

	masters := view.SelectFromMachine(func(m db.Machine) bool {
		return m.ClusterID == clusterID && m.Role == db.Master
	})
	workers := view.SelectFromMachine(func(m db.Machine) bool {
		return m.ClusterID == clusterID && m.Role == db.Worker
	})

	masters = db.SortMachines(masters)
	workers = db.SortMachines(workers)

	var changes []db.Machine

	nBoot := masterCount + workerCount - len(masters) - len(workers)

	if len(masters) > masterCount {
		changes = append(changes, masters[masterCount:]...)
		masters = masters[:masterCount]
	}

	if len(workers) > workerCount {
		changes = append(changes, workers[workerCount:]...)
		workers = workers[:workerCount]
	}

	for i := 0; i < nBoot; i++ {
		changes = append(changes, view.InsertMachine())
	}

	newWorkers := workerCount - len(workers)
	newMasters := masterCount - len(masters)
	for i := range changes {
		change := changes[i]
		change.ClusterID = clusterID
		if newMasters > 0 {
			newMasters--
			change.Role = db.Master
			view.Commit(change)
		} else if newWorkers > 0 {
			newWorkers--
			change.Role = db.Worker
			view.Commit(change)
		} else {
			view.Remove(change)
		}
	}

	return nil
}

func containerTxn(view db.Database, dsl dsl.Dsl, clusterID int) error {
	containers := view.SelectFromContainer(func(c db.Container) bool {
		return c.ClusterID == clusterID
	})

	dslContainers := dsl.QueryContainers()
	if len(containers) > len(dslContainers) {
		for _, c := range containers[len(dslContainers):] {
			view.Remove(c)
		}
		containers = containers[:len(dslContainers)]
	}

	for len(containers) < len(dslContainers) {
		containers = append(containers, view.InsertContainer())
	}

	// Make sure the container assignments are consistent across runs.
	containers = db.SortContainers(containers)

	for i, dc := range dslContainers {
		c := containers[i]
		if c.IP != "" || c.SchedID != "" {
			// This code is wrong if these values are meaningful
			panic("Not Reached")
		}

		c.ClusterID = clusterID
		c.Image = dc.Image
		c.Labels = dc.Labels
		view.Commit(c)
	}

	return nil
}

func resolveACLs(acls []string) []string {
	var result []string
	for _, acl := range acls {
		if acl == "local" {
			ip, err := myIP()
			if err != nil {
				log.Warning(err.Error())
				continue
			}
			acl = ip + "/32"
		}
		result = append(result, acl)
	}

	return result
}
