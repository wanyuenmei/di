package engine

import (
	"fmt"

	"github.com/NetSys/di/db"
	"github.com/NetSys/di/dsl"
	"github.com/NetSys/di/util"

	log "github.com/Sirupsen/logrus"
)

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

	return nil
}

func clusterTxn(view db.Database, _dsl dsl.Dsl) (int, error) {
	Namespace := _dsl.QueryString("Namespace")
	if Namespace == "" {
		return 0, fmt.Errorf("Policy must specify a 'Namespace'")
	}

	provider, err := db.ParseProvider(_dsl.QueryString("Provider"))
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
	cluster.AdminACL = resolveACLs(_dsl.QueryStrSlice("AdminACL"))
	cluster.SSHKeys = dsl.ParseKeys(_dsl.QueryKeySlice("sshkeys"))
	cluster.Spec = _dsl.String()
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

func resolveACLs(acls []string) []string {
	var result []string
	for _, acl := range acls {
		if acl == "local" {
			ip, err := myIP()
			if err != nil {
				log.WithError(err).Warn("Failed to get IP address.")
				continue
			}
			acl = ip + "/32"
		}
		result = append(result, acl)
	}

	return result
}
