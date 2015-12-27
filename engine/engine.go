package engine

import (
	"fmt"
	"sort"

	"github.com/NetSys/di/db"
	"github.com/NetSys/di/dsl"
	"github.com/NetSys/di/util"
	"github.com/op/go-logging"
)

var log = logging.MustGetLogger("engine")

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

func updateTxn(snapshot db.Database, dsl dsl.Dsl) error {
	Namespace := dsl.QueryString("Namespace")
	if Namespace == "" {
		return fmt.Errorf("Policy must specify a 'Namespace'")
	}

	provider, err := db.ParseProvider(dsl.QueryString("Provider"))
	if err != nil {
		return err
	}

	var cluster db.Cluster
	clusters := snapshot.SelectFromCluster(nil)
	switch len(clusters) {
	case 1:
		cluster = clusters[0]
	case 0:
		cluster = snapshot.InsertCluster()
	default:
		panic("Unimplemented")
	}

	cluster.Provider = provider
	cluster.Namespace = Namespace
	cluster.RedCount = dsl.QueryInt("RedCount")
	cluster.BlueCount = dsl.QueryInt("BlueCount")
	cluster.AdminACL = resolveACLs(dsl.QueryStrSlice("AdminACL"))
	cluster.SSHKeys = dsl.QueryStrSlice("SSHKeys")

	cluster.Write()

	masterCount := dsl.QueryInt("MasterCount")
	workerCount := dsl.QueryInt("WorkerCount")
	if masterCount == 0 || workerCount == 0 {
		masterCount = 0
		workerCount = 0
	}

	clusterID := cluster.ID
	masters := snapshot.SelectFromMachine(func(m db.Machine) bool {
		return m.ClusterID == clusterID && m.Role == db.Master
	})
	workers := snapshot.SelectFromMachine(func(m db.Machine) bool {
		return m.ClusterID == clusterID && m.Role == db.Worker
	})

	mSort(masters).sort()
	mSort(workers).sort()

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
		changes = append(changes, snapshot.InsertMachine())
	}

	newWorkers := workerCount - len(workers)
	newMasters := masterCount - len(masters)
	for i := range changes {
		change := &changes[i]
		change.ClusterID = clusterID
		if newMasters > 0 {
			newMasters--
			change.Role = db.Master
			change.Write()
		} else if newWorkers > 0 {
			newWorkers--
			change.Role = db.Worker
			change.Write()
		} else {
			change.Remove()
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

var myIP = util.MyIp

type mSort []db.Machine

func (machines mSort) sort() {
	sort.Stable(machines)
}

func (machines mSort) Len() int {
	return len(machines)
}

func (machines mSort) Swap(i, j int) {
	machines[i], machines[j] = machines[j], machines[i]
}

func (machines mSort) Less(i, j int) bool {
	I, J := machines[i], machines[j]

	upI := I.PublicIP != "" && I.PrivateIP != ""
	upJ := J.PublicIP != "" && J.PrivateIP != ""
	downI := I.PublicIP == "" && I.PrivateIP == ""
	downJ := J.PublicIP == "" && J.PrivateIP == ""

	switch {
	case upI != upJ:
		return upI
	case downI != downJ:
		return !downI
	default:
		return I.ID < J.ID
	}
}
