package engine

import (
	"fmt"

	"github.com/NetSys/di/db"
	"github.com/NetSys/di/dsl"
	"github.com/NetSys/di/join"
	"github.com/NetSys/di/provider"
	"github.com/NetSys/di/util"

	log "github.com/Sirupsen/logrus"
)

var myIP = util.MyIP

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

func clusterTxn(view db.Database, dsl dsl.Dsl) (int, error) {
	Namespace := dsl.QueryString("Namespace")
	if Namespace == "" {
		return 0, fmt.Errorf("policy must specify a 'Namespace'")
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

	cluster.Namespace = Namespace
	cluster.AdminACL = resolveACLs(dsl.QueryStrSlice("AdminACL"))
	cluster.SSHKeys = dsl.QueryKeySlice("sshkeys")
	cluster.Spec = dsl.String()
	view.Commit(cluster)

	return cluster.ID, nil
}

// toDBMachine converts machines specified in the DSL into db.Machines that can
// be compared against what's already in the db.
// Specifically, it sets the role of the db.Machine, the size (which may depend
// on RAM and CPU constraints), and the provider.
// Additionally, it skips machines with invalid sizes or providers
func toDBMachine(machines []dsl.Machine, role db.Role, maxPrice float64) []db.Machine {
	var dbMachines []db.Machine
	for _, dslm := range machines {
		var m db.Machine
		m.Role = role
		p, err := db.ParseProvider(dslm.Provider)
		if err != nil {
			log.WithError(err).Error("Error parsing provider.")
			continue
		}
		m.Provider = p
		if dslm.Size != "" {
			m.Size = dslm.Size
		} else {
			providerInst := provider.New(p)
			m.Size = providerInst.PickBestSize(dslm.RAM, dslm.CPU, maxPrice)
		}
		if m.Size == "" {
			log.Errorf("No valid size for %v, skipping.", m)
			continue
		}
		dbMachines = append(dbMachines, m)
	}
	return dbMachines
}

func machineTxn(view db.Database, dsl dsl.Dsl, clusterID int) error {
	// XXX: How best to deal with machines that don't specify enough information?
	dslMasters := dsl.QueryMachineSlice("masters")
	dslWorkers := dsl.QueryMachineSlice("workers")
	maxPrice, _ := dsl.QueryFloat("MaxPrice")

	var dslMachines []db.Machine
	if len(dslMasters) == 0 || len(dslWorkers) == 0 {
		dslMachines = []db.Machine{}
	} else {
		dslMachines = append(toDBMachine(dslMasters, db.Master, maxPrice), toDBMachine(dslWorkers, db.Worker, maxPrice)...)
	}

	dbMachines := view.SelectFromMachine(func(m db.Machine) bool {
		return m.ClusterID == clusterID
	})

	scoreFun := func(left, right interface{}) int {
		dslMachine := left.(db.Machine)
		dbMachine := right.(db.Machine)

		switch {
		case dslMachine.Provider != dslMachine.Provider:
			return -1
		case dbMachine.Size != "" && dslMachine.Size != dbMachine.Size:
			return -1
		case dbMachine.Role != db.None && dbMachine.Role != dslMachine.Role:
			return -1
		case dbMachine.PrivateIP == "":
			return 2
		case dbMachine.PublicIP == "":
			return 1
		default:
			return 0
		}
	}

	pairs, bootList, terminateList := join.Join(dslMachines, dbMachines, scoreFun)

	for _, toTerminate := range terminateList {
		toTerminate := toTerminate.(db.Machine)
		view.Remove(toTerminate)
	}

	for _, bootSet := range bootList {
		bootSet := bootSet.(db.Machine)

		pairs = append(pairs, join.Pair{L: bootSet, R: view.InsertMachine()})
	}

	for _, pair := range pairs {
		dslMachine := pair.L.(db.Machine)
		dbMachine := pair.R.(db.Machine)

		dbMachine.Role = dslMachine.Role
		dbMachine.Size = dslMachine.Size
		dbMachine.Provider = dslMachine.Provider
		dbMachine.ClusterID = clusterID
		view.Commit(dbMachine)
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
