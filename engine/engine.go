package engine

import (
	"fmt"
	"sort"

	"github.com/NetSys/quilt/db"
	"github.com/NetSys/quilt/dsl"
	"github.com/NetSys/quilt/join"
	"github.com/NetSys/quilt/provider"
	"github.com/NetSys/quilt/util"

	log "github.com/Sirupsen/logrus"
)

var myIP = util.MyIP
var defaultDiskSize = 32

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

	// We can't process the ACLs with the rest of the cluster fields
	// because this must occur after the cloud machines are synced with
	// the database. If we didn't, inter-machine ACLs would get removed
	// when the Quilt controller restarts, even if there are running cloud
	// machines that still need to communicate.
	if err = aclTxn(view, dsl, cluster); err != nil {
		return err
	}

	return nil
}

func clusterTxn(view db.Database, dsl dsl.Dsl) (int, error) {
	namespace := dsl.QueryString("Namespace")
	if namespace == "" {
		namespace = "DEFAULT_NAMESPACE"
		msg := "policy did not specify 'Namespace', defaulting to '%s'"
		log.Warn(fmt.Sprintf(msg, namespace))
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

	cluster.Namespace = namespace
	cluster.Spec = dsl.String()
	view.Commit(cluster)
	return cluster.ID, nil
}

func aclTxn(view db.Database, dsl dsl.Dsl, clusterID int) error {
	clusters := view.SelectFromCluster(func(c db.Cluster) bool {
		return c.ID == clusterID
	})

	if len(clusters) == 0 {
		return fmt.Errorf("could not find cluster with ID %d", clusterID)
	}

	cluster := clusters[0]
	machines := view.SelectFromMachine(func(m db.Machine) bool {
		return m.ClusterID == cluster.ID && m.PublicIP != ""
	})
	acls := resolveACLs(dsl.QueryStrSlice("AdminACL"))

	for _, m := range machines {
		acls = append(acls, m.PublicIP+"/32")
	}

	sort.Strings(acls)
	cluster.ACLs = acls
	view.Commit(cluster)

	return nil
}

// toDBMachine converts machines specified in the DSL into db.Machines that can
// be compared against what's already in the db.
// Specifically, it sets the role of the db.Machine, the size (which may depend
// on RAM and CPU constraints), and the provider.
// Additionally, it skips machines with invalid roles, sizes or providers.
func toDBMachine(machines []dsl.Machine, maxPrice float64) []db.Machine {
	var dbMachines []db.Machine
	for _, dslm := range machines {
		var m db.Machine

		role, err := db.ParseRole(dslm.Role)
		if err != nil {
			log.WithError(err).Error("Error parsing role.")
			continue
		}
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
			m.Size = providerInst.ChooseSize(dslm.RAM, dslm.CPU, maxPrice)
		}
		if m.Size == "" {
			log.Errorf("No valid size for %v, skipping.", m)
			continue
		}

		m.DiskSize = dslm.DiskSize
		if m.DiskSize == 0 {
			m.DiskSize = defaultDiskSize
		}

		m.SSHKeys = dslm.SSHKeys
		m.Region = dslm.Region
		dbMachines = append(dbMachines, m)
	}
	return dbMachines
}

func machineTxn(view db.Database, dsl dsl.Dsl, clusterID int) error {
	// XXX: How best to deal with machines that don't specify enough information?
	dslMachinesRaw := dsl.QueryMachines()
	maxPrice, _ := dsl.QueryFloat("MaxPrice")

	var dslMachines = toDBMachine(dslMachinesRaw, maxPrice)
	hasMaster := false
	hasWorker := false

	for _, machine := range dslMachines {
		hasMaster = hasMaster || machine.Role == db.Master
		hasWorker = hasWorker || machine.Role == db.Worker
	}

	if !hasMaster || !hasWorker {
		dslMachines = []db.Machine{}

		if hasMaster != hasWorker {

			if hasMaster {
				log.Warning("A master machine was specified but worker machines were not.")
			} else {
				log.Warning("Worker machine(s) were specified but a master machine was not.")
			}

		}
	}

	dbMachines := view.SelectFromMachine(func(m db.Machine) bool {
		return m.ClusterID == clusterID
	})

	scoreFun := func(left, right interface{}) int {
		dslMachine := left.(db.Machine)
		dbMachine := right.(db.Machine)

		switch {
		case dbMachine.Provider != dslMachine.Provider:
			return -1
		case dbMachine.Region != dslMachine.Region:
			return -1
		case dbMachine.Size != "" && dslMachine.Size != dbMachine.Size:
			return -1
		case dbMachine.Role != db.None && dbMachine.Role != dslMachine.Role:
			return -1
		case dbMachine.DiskSize != dslMachine.DiskSize:
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
		dbMachine.DiskSize = dslMachine.DiskSize
		dbMachine.Provider = dslMachine.Provider
		dbMachine.Region = dslMachine.Region
		dbMachine.SSHKeys = dslMachine.SSHKeys
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
