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
	"github.com/op/go-logging"
)

var log = logging.MustGetLogger("network")

const LabelMac = "0A:00:00:00:00:00"
const LSwitch = "di"
const DIBridge = "di-int"
const OVNBridge = "br-int"

func Run(conn db.Conn, store consensus.Store, dk docker.Client) {
	go readStoreRun(conn, store)
	go writeStoreRun(conn, store)

	// XXX: This initialized map is used to keep track of which containers running on
	// the local system have been set up for networking.  We shouldn't need this map,
	// instead we should simply check what's undone and do it.
	var initialized = make(map[string]struct{})

	for range conn.TriggerTick(30, db.MinionTable, db.ContainerTable,
		db.ConnectionTable, db.LabelTable, db.EtcdTable).C {
		runWorker(conn, dk, initialized)
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
		log.Warning("Failed to connect to OVSDB: %s", err)
		return
	}
	defer ovsdb.Close()

	ovsdb.CreateSwitch(LSwitch)
	lportSlice, err := ovsdb.ListPorts(LSwitch)
	if err != nil {
		log.Warning("Failed to list OVN ports: %s", err)
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

		log.Info("New logical port for %s with IP %s", dbl.Label, dbl.IP)
		err := ovsdb.CreatePort(LSwitch, dbl.Label, LabelMac, dbl.IP)
		if err != nil {
			log.Warning("Failed to create port %s: %s", dbl.Label, err)
		}
	}

	for _, dbc := range containers {
		if _, ok := garbageMap[dbc.SchedID]; ok {
			delete(garbageMap, dbc.SchedID)
			continue
		}

		log.Info("New logical port for %s with IP %s", dbc.SchedID, dbc.IP)
		err := ovsdb.CreatePort("di", dbc.SchedID, dbc.Mac, dbc.IP)
		if err != nil {
			log.Warning("Failed to create port %s: %s", dbc.SchedID, err)
		}
	}

	// Ports still in the map don't have a corresponding label otherwise they would
	// have been deleted in the preceding loop.
	for lport := range garbageMap {
		log.Info("Delete logical port %s", lport)
		if err := ovsdb.DeletePort(LSwitch, lport); err != nil {
			log.Warning("Failed to delete logical port: %s", err)
		}
	}
}
