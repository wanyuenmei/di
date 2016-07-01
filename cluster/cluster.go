package cluster

import (
	"time"

	"github.com/NetSys/quilt/cluster/provider"
	"github.com/NetSys/quilt/db"
	"github.com/NetSys/quilt/join"
	log "github.com/Sirupsen/logrus"
)

var sleep = time.Sleep

// Store the providers in a variable so we can change it in the tests
var allProviders = []db.Provider{db.Amazon, db.Azure, db.Google, db.Vagrant}

type cluster struct {
	conn    db.Conn
	trigger db.Trigger
	fm      foreman

	namespace string
	providers map[db.Provider]provider.Provider
}

// Run continually checks 'conn' for cluster changes and recreates the cluster as
// needed.
func Run(conn db.Conn) {
	var clst *cluster
	for range conn.TriggerTick(60, db.ClusterTable).C {
		var dbCluster db.Cluster
		err := conn.Transact(func(db db.Database) error {
			var err error
			dbCluster, err = db.GetCluster()
			return err
		})

		if err == nil && clst.namespace != dbCluster.Namespace {
			if clst != nil {
				clst.fm.stop()
				clst.trigger.Stop()
			}
			clst = newCluster(conn, dbCluster.Namespace)
			go clst.listen()
		}
	}
}

func newCluster(conn db.Conn, namespace string) *cluster {
	clst := &cluster{
		conn:      conn,
		trigger:   conn.TriggerTick(30, db.ClusterTable, db.MachineTable),
		fm:        createForeman(conn),
		namespace: namespace,
		providers: make(map[db.Provider]provider.Provider),
	}

	for _, p := range allProviders {
		inst := provider.New(p)
		if err := inst.Connect(namespace); err == nil {
			clst.providers[p] = inst
		} else {
			log.Debugf("Failed to connect to provider %s: %s", p, err)
		}
	}

	return clst
}

func (clst *cluster) listen() {
	rateLimit := time.NewTicker(5 * time.Second)
	defer rateLimit.Stop()

	clst.sync()
	clst.fm.init()
	for range clst.trigger.C {
		<-rateLimit.C
		clst.sync()
		clst.fm.runOnce()
	}
}

func (clst cluster) get() ([]provider.Machine, error) {
	var cloudMachines []provider.Machine
	for _, p := range clst.providers {
		providerMachines, err := p.List()
		if err != nil {
			return []provider.Machine{}, err
		}
		cloudMachines = append(cloudMachines, providerMachines...)
	}
	return cloudMachines, nil
}

func (clst cluster) updateCloud(machines []provider.Machine, boot bool) {
	if len(machines) == 0 {
		return
	}

	actionString := "halt"
	if boot {
		actionString = "boot"
	}

	log.WithField("count", len(machines)).
		Infof("Attempt to %s machines.", actionString)

	noFailures := true
	groupedMachines := provider.GroupBy(machines)
	for p, providerMachines := range groupedMachines {
		providerInst, ok := clst.providers[p]
		if !ok {
			noFailures = false
			log.Warnf("Provider %s is unavailable.", p)
			continue
		}
		var err error
		if boot {
			err = providerInst.Boot(providerMachines)
		} else {
			err = providerInst.Stop(providerMachines)
		}
		if err != nil {
			noFailures = false
			log.WithError(err).
				Warnf("Unable to %s machines on %s.", actionString, p)
		}
	}

	if noFailures {
		log.Infof("Successfully %sed machines.", actionString)
	} else {
		log.Infof("Due to failures, sleeping for 1 minute")
		sleep(60 * time.Second)
	}
}

func (clst cluster) sync() {
	var acls []string
	var machines []db.Machine
	clst.conn.Transact(func(view db.Database) error {
		dbCluster, _ := view.GetCluster()
		machines = view.SelectFromMachine(nil)
		acls = dbCluster.ACLs
		return nil
	})

	clst.syncACLs(acls, machines)

	/* Each iteration of this loop does the following:
	 *
	 * - Get the current set of machines from the cloud provider.
	 * - Get the current policy from the database.
	 * - Compute a diff.
	 * - Update the cloud provider accordingly.
	 *
	 * Updating the cloud provider may have consequences (creating machines for
	 * instances) that should be reflected in the database.  Therefore, if updates
	 * are necessary the code loops so that database can be updated before
	 * the next sync() call. */
	for i := 0; i < 3; i++ {
		cloudMachines, err := clst.get()
		if err != nil {
			log.WithError(err).Error("Failed to list machines.")
			return
		}

		var dbMachines []db.Machine
		clst.conn.Transact(func(view db.Database) error {
			dbMachines = view.SelectFromMachine(nil)
			return nil
		})

		pairs, bootSet, terminateSet := syncDB(cloudMachines, dbMachines)

		clst.conn.Transact(func(view db.Database) error {
			for _, pair := range pairs {
				dbm := pair.L.(db.Machine)
				m := pair.R.(provider.Machine)

				dbm.CloudID = m.ID
				dbm.PublicIP = m.PublicIP
				dbm.PrivateIP = m.PrivateIP

				// If we overwrite the machine's size before the machine
				// has fully booted, the Stitch will flip it back
				// immediately.
				if m.Size != "" {
					dbm.Size = m.Size
				}
				if m.DiskSize != 0 {
					dbm.DiskSize = m.DiskSize
				}
				dbm.Provider = m.Provider
				// XXX: Get the SSH keys?
				view.Commit(dbm)
			}
			return nil
		})

		clst.updateCloud(bootSet, true)
		clst.updateCloud(terminateSet, false)
		sleep(5 * time.Second)
	}
}

func (clst cluster) syncACLs(acls []string, machines []db.Machine) {
	// Providers with at least one machine.
	prvdrSet := map[db.Provider]struct{}{}
	for _, m := range machines {
		prvdrSet[m.Provider] = struct{}{}
	}

	for name, provider := range clst.providers {
		// For this providers with no specified machines, we remove all ACLs.
		// Otherwise we set acls to what's specified.
		var setACLs []string
		if _, ok := prvdrSet[name]; ok {
			setACLs = acls
		}

		if err := provider.SetACLs(setACLs); err != nil {
			log.WithError(err).Warnf("Could not update ACLs on %s.", name)
		}
	}
}

func syncDB(cloudMachines []provider.Machine, dbMachines []db.Machine) (
	pairs []join.Pair, bootSet []provider.Machine, terminateSet []provider.Machine) {
	scoreFun := func(left, right interface{}) int {
		dbm := left.(db.Machine)
		m := right.(provider.Machine)

		switch {
		case dbm.Provider != m.Provider:
			return -1
		case m.Region != "" && dbm.Region != m.Region:
			return -1
		case m.Size != "" && dbm.Size != m.Size:
			return -1
		case m.DiskSize != 0 && dbm.DiskSize != m.DiskSize:
			return -1
		case dbm.CloudID == m.ID:
			return 0
		case dbm.PublicIP == m.PublicIP:
			return 1
		case dbm.PrivateIP == m.PrivateIP:
			return 2
		default:
			return 3
		}
	}

	pairs, dbmIface, cmIface := join.Join(dbMachines, cloudMachines, scoreFun)

	for _, cm := range cmIface {
		m := cm.(provider.Machine)
		terminateSet = append(terminateSet, m)
	}

	for _, dbm := range dbmIface {
		m := dbm.(db.Machine)
		bootSet = append(bootSet, provider.Machine{
			Size:     m.Size,
			Provider: m.Provider,
			Region:   m.Region,
			DiskSize: m.DiskSize,
			SSHKeys:  m.SSHKeys})
	}

	return pairs, bootSet, terminateSet
}
