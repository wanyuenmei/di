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

	var reds, blues []db.Container
	for _, container := range containers {
		if len(container.Labels) != 1 {
			panic("Unimplemented")
		}

		switch container.Labels[0] {
		case "Red":
			reds = append(reds, container)
		case "Blue":
			blues = append(blues, container)
		default:
			view.Remove(container)
		}

	}

	redCount := dsl.QueryInt("RedCount")
	if len(reds) > redCount {
		for _, container := range reds[redCount:] {
			view.Remove(container)
		}
	} else {
		for i := 0; i < redCount-len(reds); i++ {
			container := view.InsertContainer()
			container.ClusterID = clusterID
			container.Labels = []string{"Red"}
			container.Image = "alpine"
			view.Commit(container)
		}
	}

	blueCount := dsl.QueryInt("BlueCount")
	if len(blues) > blueCount {
		for _, container := range blues[blueCount:] {
			view.Remove(container)
		}
	} else {
		for i := 0; i < blueCount-len(blues); i++ {
			container := view.InsertContainer()
			container.ClusterID = clusterID
			container.Labels = []string{"Blue"}
			container.Image = "alpine"
			view.Commit(container)
		}
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
