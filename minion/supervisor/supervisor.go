package supervisor

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/NetSys/di/db"
	"github.com/NetSys/di/minion/docker"
	"github.com/op/go-logging"
)

var log = logging.MustGetLogger("supervisor")

const (
	etcd          = "etcd"
	swarm         = "swarm"
	ovnnorthd     = "ovn-northd"
	ovncontroller = "ovn-controller"
	ovnoverlay    = "ovn-overlay"
	ovsvswitchd   = "ovs-vswitchd"
	ovsdb         = "ovsdb-server"
)

var images = map[string]string{
	etcd:          "quay.io/coreos/etcd:v2.2.4",
	ovncontroller: "quay.io/netsys/ovn-controller",
	ovnnorthd:     "quay.io/netsys/ovn-northd",
	ovnoverlay:    "quay.io/netsys/ovn-overlay",
	ovsdb:         "quay.io/netsys/ovsdb-server",
	ovsvswitchd:   "quay.io/netsys/ovs-vswitchd",
	swarm:         "swarm:1.0.1",
}

const etcdHeartbeatInterval = "500"
const etcdElectionTimeout = "5000"

type supervisor struct {
	conn db.Conn
	dk   docker.Client

	role     db.Role
	etcdIPs  []string
	leaderIP string
	IP       string
	leader   bool
}

func Run(conn db.Conn, dk docker.Client) {
	sv := supervisor{conn: conn, dk: dk}

	for _, image := range images {
		go sv.dk.Pull(image)
	}

	for range conn.Trigger(db.MinionTable, db.EtcdTable).C {
		sv.runOnce()
	}
}

func (sv *supervisor) runOnce() {
	var minion db.Minion
	var etcdRow db.Etcd
	minions := sv.conn.SelectFromMinion(nil)
	etcdRows := sv.conn.SelectFromEtcd(nil)
	if len(minions) == 1 {
		minion = minions[0]
	}
	if len(etcdRows) == 1 {
		etcdRow = etcdRows[0]
	}

	if sv.role == minion.Role &&
		reflect.DeepEqual(sv.etcdIPs, etcdRow.EtcdIPs) &&
		sv.leaderIP == etcdRow.LeaderIP &&
		sv.IP == minion.PrivateIP &&
		sv.leader == etcdRow.Leader {
		return
	}

	if minion.Role != sv.role {
		sv.RemoveAll()
	}

	switch minion.Role {
	case db.Master:
		sv.updateMaster(minion.PrivateIP, etcdRow.EtcdIPs,
			etcdRow.Leader)
	case db.Worker:
		sv.updateWorker(minion.PrivateIP, etcdRow.LeaderIP,
			etcdRow.EtcdIPs)
	}

	sv.role = minion.Role
	sv.etcdIPs = etcdRow.EtcdIPs
	sv.leaderIP = etcdRow.LeaderIP
	sv.IP = minion.PrivateIP
	sv.leader = etcdRow.Leader
}

func (sv *supervisor) updateWorker(IP string, leaderIP string, etcdIPs []string) {
	if !reflect.DeepEqual(sv.etcdIPs, etcdIPs) {
		sv.Remove(etcd)
	}

	if sv.leaderIP != leaderIP || sv.IP != IP {
		sv.Remove(swarm)
	}

	sv.run(etcd, fmt.Sprintf("--initial-cluster=%s", initialClusterString(etcdIPs)),
		"--heartbeat-interval="+etcdHeartbeatInterval,
		"--election-timeout="+etcdElectionTimeout,
		"--proxy=on")

	sv.run(ovsdb)
	sv.run(ovsvswitchd)

	if leaderIP == "" || IP == "" {
		return
	}

	sv.run(swarm, "join", fmt.Sprintf("--addr=%s:2375", IP), "etcd://127.0.0.1:2379")

	minions := sv.conn.SelectFromMinion(nil)
	if len(minions) != 1 {
		return
	}

	err := sv.dk.Exec(ovsvswitchd, "ovs-vsctl", "set", "Open_vSwitch", ".",
		fmt.Sprintf("external_ids:ovn-remote=\"tcp:%s:6640\"", leaderIP),
		fmt.Sprintf("external_ids:ovn-encap-ip=%s", IP),
		"external_ids:ovn-encap-type=\"geneve\"",
		fmt.Sprintf("external_ids:api_server=\"http://%s:9000\"", leaderIP),
		fmt.Sprintf("external_ids:system-id=\"di-%s\"", minions[0].MinionID))
	if err != nil {
		log.Warning("Failed to exec in %s: %s", ovsvswitchd, err)
	}

	/* The ovn controller doesn't support reconfiguring ovn-remote mid-run.
	 * So, we need to restart the container when the leader changes. */
	sv.Remove(ovncontroller)
	sv.run(ovncontroller)

	sv.run(ovnoverlay)
}

func (sv *supervisor) updateMaster(IP string, etcdIPs []string, leader bool) {
	if sv.IP != IP || !reflect.DeepEqual(sv.etcdIPs, etcdIPs) {
		sv.Remove(etcd)
	}

	if sv.IP != IP {
		sv.Remove(swarm)
	}

	if IP == "" || len(etcdIPs) == 0 {
		return
	}

	sv.run(etcd, fmt.Sprintf("--name=master-%s", IP),
		fmt.Sprintf("--initial-cluster=%s", initialClusterString(etcdIPs)),
		fmt.Sprintf("--advertise-client-urls=http://%s:2379", IP),
		fmt.Sprintf("--listen-peer-urls=http://%s:2380", IP),
		fmt.Sprintf("--initial-advertise-peer-urls=http://%s:2380", IP),
		"--listen-client-urls=http://0.0.0.0:2379",
		"--heartbeat-interval="+etcdHeartbeatInterval,
		"--initial-cluster-state=new",
		"--election-timeout="+etcdElectionTimeout)
	sv.run(ovsdb)

	swarmAddr := IP + ":2377"
	sv.run(swarm, "manage", "--replication", "--addr="+swarmAddr,
		"--host="+swarmAddr, "etcd://127.0.0.1:2379")

	if leader {
		/* XXX: If we fail to boot ovn-northd, we should give up
		* our leadership somehow.  This ties into the general
		* problem of monitoring health. */
		sv.run(ovnnorthd)
	} else {
		sv.Remove(ovnnorthd)
	}
}

func (sv *supervisor) run(name string, args ...string) {
	ro := docker.RunOptions{
		Name:        name,
		Image:       images[name],
		Args:        args,
		NetworkMode: "host",
	}

	switch name {
	case ovnoverlay:
		ro.Binds = []string{"/etc/docker:/etc/docker:rw"}
		fallthrough
	case ovsvswitchd:
		ro.Privileged = true
		fallthrough
	case ovnnorthd:
		fallthrough
	case ovncontroller:
		ro.VolumesFrom = []string{ovsdb}
	case etcd:
		fallthrough
	case ovsdb:
		ro.Binds = []string{"/usr/share/ca-certificates:/etc/ssl/certs"}
	}

	if err := sv.dk.Run(ro); err != nil {
		log.Warning("Failed to run %s: %s", name, err)
	}
}

func (sv *supervisor) Remove(name string) {
	if err := sv.dk.Remove(name); err != nil {
		log.Warning("Failed to remove %s: %s", name, err)
	}
}

func (sv *supervisor) RemoveAll() {
	for name := range images {
		sv.Remove(name)
	}
}

func initialClusterString(etcdIPs []string) string {
	var initialCluster []string
	for _, ip := range etcdIPs {
		initialCluster = append(initialCluster, fmt.Sprintf("%s=http://%s:2380", nodeName(ip), ip))
	}
	return strings.Join(initialCluster, ",")
}

func nodeName(IP string) string {
	return fmt.Sprintf("master-%s", IP)
}
