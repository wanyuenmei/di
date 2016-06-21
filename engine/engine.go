package engine

import (
	"fmt"
	"sort"

	"github.com/NetSys/quilt/cluster/provider"
	"github.com/NetSys/quilt/db"
	"github.com/NetSys/quilt/join"
	"github.com/NetSys/quilt/stitch"
	"github.com/NetSys/quilt/util"

	log "github.com/Sirupsen/logrus"
)

var myIP = util.MyIP
var defaultDiskSize = 32

// UpdatePolicy executes transactions on 'conn' to make it reflect a new policy,
// 'stitch'.
func UpdatePolicy(conn db.Conn, stitch stitch.Stitch) error {
	txn := func(db db.Database) error {
		return updateTxn(db, stitch)
	}

	if err := conn.Transact(txn); err != nil {
		return err
	}

	return nil
}

func updateTxn(view db.Database, stitch stitch.Stitch) error {
	cluster, err := clusterTxn(view, stitch)
	if err != nil {
		return err
	}

	if err = machineTxn(view, stitch, cluster); err != nil {
		return err
	}

	// We can't process the ACLs with the rest of the cluster fields
	// because this must occur after the cloud machines are synced with
	// the database. If we didn't, inter-machine ACLs would get removed
	// when the Quilt controller restarts, even if there are running cloud
	// machines that still need to communicate.
	if err = aclTxn(view, stitch, cluster); err != nil {
		return err
	}

	return nil
}

func clusterTxn(view db.Database, stitch stitch.Stitch) (int, error) {
	namespace := stitch.QueryString("Namespace")
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
	cluster.Spec = stitch.String()
	view.Commit(cluster)
	return cluster.ID, nil
}

func aclTxn(view db.Database, stitch stitch.Stitch, clusterID int) error {
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
	acls := resolveACLs(stitch.QueryStrSlice("AdminACL"))

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
func toDBMachine(machines []stitch.Machine, maxPrice float64) []db.Machine {
	var hasMaster, hasWorker bool
	var dbMachines []db.Machine
	for _, stitchm := range machines {
		var m db.Machine

		role, err := db.ParseRole(stitchm.Role)
		if err != nil {
			log.WithError(err).Error("Error parsing role.")
			continue
		}
		m.Role = role

		hasMaster = hasMaster || role == db.Master
		hasWorker = hasWorker || role == db.Worker

		p, err := db.ParseProvider(stitchm.Provider)
		if err != nil {
			log.WithError(err).Error("Error parsing provider.")
			continue
		}
		m.Provider = p
		m.Size = stitchm.Size

		if m.Size == "" {
			providerInst := provider.New(p)
			m.Size = providerInst.ChooseSize(stitchm.RAM, stitchm.CPU, maxPrice)
			if m.Size == "" {
				log.Errorf("No valid size for %v, skipping.", m)
				continue
			}
		}

		m.DiskSize = stitchm.DiskSize
		if m.DiskSize == 0 {
			m.DiskSize = defaultDiskSize
		}

		m.SSHKeys = stitchm.SSHKeys
		m.Region = stitchm.Region
		dbMachines = append(dbMachines, provider.DefaultRegion(m))
	}

	if !hasMaster && hasWorker {
		log.Warning("A Master was specified but no workers.")
		return nil
	} else if hasMaster && !hasWorker {
		log.Warning("A Worker was specified but no masters.")
		return nil
	}

	return dbMachines
}

func machineTxn(view db.Database, stitch stitch.Stitch, clusterID int) error {
	// XXX: How best to deal with machines that don't specify enough information?
	maxPrice, _ := stitch.QueryFloat("MaxPrice")
	stitchMachines := toDBMachine(stitch.QueryMachines(), maxPrice)

	dbMachines := view.SelectFromMachine(func(m db.Machine) bool {
		return m.ClusterID == clusterID
	})

	scoreFun := func(left, right interface{}) int {
		stitchMachine := left.(db.Machine)
		dbMachine := right.(db.Machine)

		switch {
		case dbMachine.Provider != stitchMachine.Provider:
			return -1
		case dbMachine.Region != stitchMachine.Region:
			return -1
		case dbMachine.Size != "" && stitchMachine.Size != dbMachine.Size:
			return -1
		case dbMachine.Role != db.None && dbMachine.Role != stitchMachine.Role:
			return -1
		case dbMachine.DiskSize != stitchMachine.DiskSize:
			return -1
		case dbMachine.PrivateIP == "":
			return 2
		case dbMachine.PublicIP == "":
			return 1
		default:
			return 0
		}
	}

	pairs, bootList, terminateList := join.Join(stitchMachines, dbMachines, scoreFun)

	for _, toTerminate := range terminateList {
		toTerminate := toTerminate.(db.Machine)
		view.Remove(toTerminate)
	}

	for _, bootSet := range bootList {
		bootSet := bootSet.(db.Machine)

		pairs = append(pairs, join.Pair{L: bootSet, R: view.InsertMachine()})
	}

	for _, pair := range pairs {
		stitchMachine := pair.L.(db.Machine)
		dbMachine := pair.R.(db.Machine)

		dbMachine.Role = stitchMachine.Role
		dbMachine.Size = stitchMachine.Size
		dbMachine.DiskSize = stitchMachine.DiskSize
		dbMachine.Provider = stitchMachine.Provider
		dbMachine.Region = stitchMachine.Region
		dbMachine.SSHKeys = stitchMachine.SSHKeys
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
