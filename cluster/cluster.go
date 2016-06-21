package cluster

import (
	"time"

	"github.com/NetSys/quilt/cluster/provider"
	"github.com/NetSys/quilt/db"
	"github.com/NetSys/quilt/join"
	log "github.com/Sirupsen/logrus"
)

var sleep = time.Sleep

type cluster struct {
	id      int
	conn    db.Conn
	trigger db.Trigger
	fm      foreman

	providers map[db.Provider]provider.Provider

	mark bool /* For mark and sweep garbage collection. */
}

// Run continually checks 'conn' for new clusters and implements the policies they
// dictate.
func Run(conn db.Conn) {
	clusters := make(map[int]*cluster)

	for range conn.TriggerTick(60, db.ClusterTable).C {
		var dbClusters []db.Cluster
		conn.Transact(func(db db.Database) error {
			dbClusters = db.SelectFromCluster(nil)
			return nil
		})

		/* Mark and sweep garbage collect the clusters. */
		for _, row := range dbClusters {
			clst, ok := clusters[row.ID]
			if !ok {
				new := newCluster(conn, row.ID, row.Namespace)
				clst = &new
				clusters[row.ID] = clst
			}

			clst.mark = true
		}

		for k, clst := range clusters {
			if clst.mark {
				clst.mark = false
			} else {
				clst.fm.stop()
				clst.trigger.Stop()
				delete(clusters, k)
			}
		}
	}
}

func newCluster(conn db.Conn, id int, namespace string) cluster {
	clst := cluster{
		id:        id,
		conn:      conn,
		trigger:   conn.TriggerTick(30, db.ClusterTable, db.MachineTable),
		fm:        newForeman(conn, id),
		providers: make(map[db.Provider]provider.Provider),
	}

	for _, p := range []db.Provider{db.Amazon, db.Google, db.Azure, db.Vagrant} {
		inst := provider.New(p)
		err := inst.Connect(namespace)
		if err == nil {
			clst.providers[p] = inst
		} else {
			log.Debugf("Failed to connect to provider %s: %s", p, err)
		}
	}
	go func() {
		rateLimit := time.NewTicker(5 * time.Second)
		defer rateLimit.Stop()
		for range clst.trigger.C {
			clst.sync()
			<-rateLimit.C
		}
		for _, p := range clst.providers {
			p.Disconnect()
		}
	}()
	return clst
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

	log.WithField("count", len(machines)).Infof("Attempt to %s machines.", actionString)

	noFailures := true
	groupedMachines := provider.GroupBy(machines)
	for p, providerMachines := range groupedMachines {
		if _, ok := clst.providers[p]; !ok {
			noFailures = false
			log.Warnf("Provider %s is unavailable.", p)
			continue
		}
		providerInst := clst.providers[p]
		var err error
		if boot {
			err = providerInst.Boot(providerMachines)
		} else {
			err = providerInst.Stop(providerMachines)
		}
		if err != nil {
			noFailures = false
			log.WithError(err).Warnf("Unable to %s machines on %s.", actionString, p)
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
		clusters := view.SelectFromCluster(func(c db.Cluster) bool {
			return c.ID == clst.id
		})

		machines = view.SelectFromMachine(func(m db.Machine) bool {
			return m.ClusterID == clst.id
		})

		if len(clusters) == 0 {
			log.Warn("Undefined cluster.")
			return nil
		} else if len(clusters) > 1 {
			panic("Duplicate Clusters")
		}

		acls = clusters[0].ACLs
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
			dbMachines = view.SelectFromMachine(func(m db.Machine) bool {
				return m.ClusterID == clst.id
			})
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
				// If we overwrite the machine's size before the machine has fully
				// booted, the DSL will flip it back immediately.
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
