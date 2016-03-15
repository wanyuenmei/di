// Package network manages the network services of the application dataplane.  This means
// ensuring that containers can find and communicate with each other in accordance with
// the policy specification.  It achieves this by manipulating IP addresses and hostnames
// within the containers, Open vSwitch on each running worker, and the OVN controller.
package network

import (
	"github.com/NetSys/di/db"
	"github.com/NetSys/di/minion/consensus"
	"github.com/NetSys/di/minion/docker"
	"github.com/NetSys/di/ovsdb"
	"github.com/NetSys/di/util"

	log "github.com/Sirupsen/logrus"
)

const labelMac = "0A:00:00:00:00:00"
const lSwitch = "di"
const diBridge = "di-int"
const ovnBridge = "br-int"

// Run blocks implementing the network services.
func Run(conn db.Conn, store consensus.Store, dk docker.Client) {
	go readStoreRun(conn, store)
	go writeStoreRun(conn, store)

	for range conn.TriggerTick(30, db.MinionTable, db.ContainerTable,
		db.ConnectionTable, db.LabelTable, db.EtcdTable).C {
		runWorker(conn, dk)
		runMaster(conn)
	}
}

// The leader of the cluster is responsible for properly configuring OVN northd for
// container networking.  This simply means creating a logical port for each container
// and label.  The specialized OpenFlow rules DI requires are managed by the workers
// individuallly.
func runMaster(conn db.Conn) {
	var etcds []db.Etcd
	var labels []db.Label
	var containers []db.Container
	conn.Transact(func(view db.Database) error {
		etcds = view.SelectFromEtcd(nil)

		labels = view.SelectFromLabel(func(label db.Label) bool {
			return label.IP != ""
		})

		containers = view.SelectFromContainer(func(dbc db.Container) bool {
			return dbc.SchedID != "" && dbc.Mac != "" && dbc.IP != ""
		})
		return nil
	})

	if len(etcds) != 1 || !etcds[0].Leader {
		return
	}

	ovsdb, err := ovsdb.Open()
	if err != nil {
		log.WithError(err).Error("Failed to connect to OVSDB.")
		return
	}
	defer ovsdb.Close()

	ovsdb.CreateSwitch(lSwitch)
	lportSlice, err := ovsdb.ListPorts(lSwitch)
	if err != nil {
		log.WithError(err).Error("Failed to list OVN ports.")
		return
	}

	// The garbageMap starts of containing every logical port in OVN.  As we find
	// that these ports are still useful, they're deleted from garbageMap until only
	// leftover garbage ports are remaining.  These are then deleted.
	garbageMap := make(map[string]struct{})
	for _, lport := range lportSlice {
		garbageMap[lport.Name] = struct{}{}
	}

	for _, dbl := range labels {
		if _, ok := garbageMap[dbl.Label]; ok {
			delete(garbageMap, dbl.Label)
			continue
		}

		log.WithFields(log.Fields{
			"name": dbl.Label,
			"IP":   dbl.IP,
		}).Info("New logical port.")
		err := ovsdb.CreatePort(lSwitch, dbl.Label, labelMac, dbl.IP)
		if err != nil {
			log.WithError(err).Warnf("Failed to create port %s.", dbl.Label)
		}
	}

	for _, dbc := range containers {
		if _, ok := garbageMap[dbc.SchedID]; ok {
			delete(garbageMap, dbc.SchedID)
			continue
		}

		log.WithFields(log.Fields{
			"name": util.ShortUUID(dbc.SchedID),
			"IP":   dbc.IP,
		}).Info("New logical port.")
		err := ovsdb.CreatePort("di", dbc.SchedID, dbc.Mac, dbc.IP)
		if err != nil {
			log.WithFields(log.Fields{
				"error": err,
				"name":  dbc.SchedID,
			}).Warn("Failed to create port.")
		}
	}

	// Ports still in the map don't have a corresponding label otherwise they would
	// have been deleted in the preceding loop.
	for lport := range garbageMap {
		log.Infof("Delete logical port %s.", lport)
		if err := ovsdb.DeletePort(lSwitch, lport); err != nil {
			log.WithError(err).Warn("Failed to delete logical port.")
		}
	}
}
