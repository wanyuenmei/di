package cluster

import (
	"github.com/NetSys/di/db"
	"github.com/NetSys/di/join"
	"github.com/NetSys/di/util"
	log "github.com/Sirupsen/logrus"
)

type machine struct {
	id        string
	publicIP  string
	privateIP string
}

type provider interface {
	get() ([]machine, error)

	boot(count int, cloudConfig string) error

	stop(ids []string) error

	disconnect()
}

type cluster struct {
	provider
	id          int
	conn        db.Conn
	cloudConfig string
	trigger     db.Trigger
	fm          foreman

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

	var cloud provider
	var err error
	var cloudConfig string

	switch dbp {
	case db.AmazonSpot:
		cloud = newAWS(conn, id, namespace)
		cloudConfig = util.CloudConfigUbuntu(keys)
	case db.Google:
		cloud, err = newGCE(conn, id, namespace)
		cloudConfig = util.CloudConfigCoreOS(keys)
	case db.Azure:
		cloud, err = newAzure(conn, id, namespace)
		cloudConfig = util.CloudConfigUbuntu(keys)
	case db.Vagrant:
		cloud, err = newVagrant(namespace)
		cloudConfig = util.CloudConfigCoreOS(append(keys, VagrantPublicKey))
	default:
		panic("Unimplemented")
	}

	if err != nil {
		return cluster{}, err
	}

	clst := cluster{
		provider:    cloud,
		id:          id,
		conn:        conn,
		cloudConfig: cloudConfig,
		trigger:     conn.TriggerTick(30, db.MachineTable),
		fm:          newForeman(conn, id),
	}

	go func() {
		defer clst.provider.disconnect()
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
		cloudMachines, err := clst.get()
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
			err := clst.boot(nBoot, clst.cloudConfig)
			if err != nil {
				log.WithError(err).Warn("Failed to boot machines.")
			} else {
				log.Info("Successfully booted machines.")
			}
		}

		if len(terminateSet) > 0 {
			log.WithField("machines", terminateSet).Info("Attempt to stop.")
			if err := clst.stop(terminateSet); err != nil {
				log.WithError(err).Error("Failed to stop machines.")
			} else {
				log.Info("Successfully stopped machines.")
			}
		}
	}
}

func (clst cluster) syncDB(cloudMachines []machine) (int, []string) {
	var nBoot int
	var terminateSet []string
	clst.conn.Transact(func(view db.Database) error {
		machines := view.SelectFromMachine(func(m db.Machine) bool {
			return m.ClusterID == clst.id
		})

		scoreFun := func(left, right interface{}) int {
			dbm := left.(db.Machine)
			m := right.(machine)

			switch {
			case dbm.CloudID == m.id:
				return 0
			case dbm.PublicIP == m.publicIP:
				return 1
			case dbm.PrivateIP == m.privateIP:
				return 2
			default:
				return 3
			}
		}

		pairs, dbmIface, cmIface := join.Join(machines, cloudMachines, scoreFun)

		for _, pair := range pairs {
			dbm := pair.L.(db.Machine)
			m := pair.R.(machine)

			dbm.CloudID = m.id
			dbm.PublicIP = m.publicIP
			dbm.PrivateIP = m.privateIP
			view.Commit(dbm)
		}

		for _, cm := range cmIface {
			m := cm.(machine)
			terminateSet = append(terminateSet, m.id)
		}

		nBoot = len(dbmIface)

		return nil
	})

	return nBoot, terminateSet
}
