package cluster

import (
	"github.com/NetSys/di/db"
	"github.com/NetSys/di/join"
	"github.com/NetSys/di/provider"
	log "github.com/Sirupsen/logrus"
)

type cluster struct {
	cloud   provider.Provider
	id      int
	conn    db.Conn
	trigger db.Trigger
	fm      foreman

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
				new, err := newCluster(conn, row.ID, row.Provider,
					row.Namespace, row.SSHKeys)
				if err != nil {
					log.WithError(err).Errorf(
						"Failed to create cluster.")
					continue
				}
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

func newCluster(conn db.Conn, id int, dbp db.Provider, namespace string,
	keys []string) (cluster, error) {

	cloud := provider.New(dbp)

	err := cloud.Start(conn, id, namespace, keys)

	if err != nil {
		return cluster{}, err
	}

	clst := cluster{
		cloud:   cloud,
		id:      id,
		conn:    conn,
		trigger: conn.TriggerTick(30, db.MachineTable),
		fm:      newForeman(conn, id),
	}

	go func() {
		defer clst.cloud.Disconnect()
		for range clst.trigger.C {
			clst.sync()
		}
	}()
	return clst, nil
}

func (clst cluster) sync() {
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
	for i := 0; i < 8; i++ {
		cloudMachines, err := clst.cloud.Get()
		if err != nil {
			log.WithError(err).Error("Failed to list machines.")
			return
		}

		nBoot, terminateSet := clst.syncDB(cloudMachines)
		if nBoot == 0 && len(terminateSet) == 0 {
			return
		}

		if nBoot > 0 {
			log.WithField("count", nBoot).Info("Attempt to boot machines.")
			err := clst.cloud.Boot(nBoot)
			if err != nil {
				log.WithError(err).Warn("Failed to boot machines.")
			} else {
				log.Info("Successfully booted machines.")
			}
		}

		if len(terminateSet) > 0 {
			log.WithField("machines", terminateSet).Info("Attempt to stop.")
			if err := clst.cloud.Stop(terminateSet); err != nil {
				log.WithError(err).Error("Failed to stop machines.")
			} else {
				log.Info("Successfully stopped machines.")
			}
		}
	}
}

func (clst cluster) syncDB(cloudMachines []provider.Machine) (int, []string) {
	var nBoot int
	var terminateSet []string
	clst.conn.Transact(func(view db.Database) error {
		machines := view.SelectFromMachine(func(m db.Machine) bool {
			return m.ClusterID == clst.id
		})

		scoreFun := func(left, right interface{}) int {
			dbm := left.(db.Machine)
			m := right.(provider.Machine)

			switch {
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

		pairs, dbmIface, cmIface := join.Join(machines, cloudMachines, scoreFun)

		for _, pair := range pairs {
			dbm := pair.L.(db.Machine)
			m := pair.R.(provider.Machine)

			dbm.CloudID = m.ID
			dbm.PublicIP = m.PublicIP
			dbm.PrivateIP = m.PrivateIP
			view.Commit(dbm)
		}

		for _, cm := range cmIface {
			m := cm.(provider.Machine)
			terminateSet = append(terminateSet, m.ID)
		}

		nBoot = len(dbmIface)

		return nil
	})

	return nBoot, terminateSet
}
