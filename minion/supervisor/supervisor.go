package supervisor

import (
	"fmt"

	"github.com/NetSys/di/db"
	"github.com/NetSys/di/minion/docker"
	"github.com/op/go-logging"
)

var log = logging.MustGetLogger("supervisor")

const (
	etcd          = "etcd"
	kubelet       = "kubelet"
	ovnnorthd     = "ovn-northd"
	ovncontroller = "ovn-controller"
	ovnoverlay    = "ovn-overlay"
	ovsvswitchd   = "ovs-vswitchd"
	ovsdb         = "ovsdb-server"
)

var images = map[string]string{
	etcd:          "quay.io/coreos/etcd:v2.2.4",
	kubelet:       "quay.io/netsys/kubelet",
	ovnnorthd:     "quay.io/netsys/ovn-northd",
	ovncontroller: "quay.io/netsys/ovn-controller",
	ovnoverlay:    "quay.io/netsys/ovn-overlay",
	ovsvswitchd:   "quay.io/netsys/ovs-vswitchd",
	ovsdb:         "quay.io/netsys/ovsdb-server",
}

const etcdHeartbeatInterval = "500"
const etcdElectionTimeout = "5000"

type supervisor struct {
	conn db.Conn
	dk   docker.Client

	role      db.Role
	etcdToken string
	leaderIP  string
	IP        string
	leader    bool
}

func Run(conn db.Conn, dk docker.Client) {
	sv := supervisor{conn: conn, dk: dk}

	for image := range images {
		go sv.dk.Pull(image)
	}

	for range conn.Trigger(db.MinionTable).C {
		sv.runOnce()
	}
}

func (sv *supervisor) runOnce() {
	var minion db.Minion
	minions := sv.conn.SelectFromMinion(nil)
	if len(minions) == 1 {
		minion = minions[0]
	}

	if sv.role == minion.Role &&
		sv.etcdToken == minion.EtcdToken &&
		sv.leaderIP == minion.LeaderIP &&
		sv.IP == minion.PrivateIP &&
		sv.leader == minion.Leader {
		return
	}

	if minion.Role != sv.role {
		sv.RemoveAll()
	}

	switch minion.Role {
	case db.Master:
		sv.updateMaster(minion.PrivateIP, minion.EtcdToken,
			minion.Leader)
	case db.Worker:
		sv.updateWorker(minion.PrivateIP, minion.LeaderIP,
			minion.EtcdToken)
	}

	sv.role = minion.Role
	sv.etcdToken = minion.EtcdToken
	sv.leaderIP = minion.LeaderIP
	sv.IP = minion.PrivateIP
	sv.leader = minion.Leader
}

func (sv *supervisor) updateWorker(IP, leaderIP, etcdToken string) {
	if sv.etcdToken != etcdToken {
		sv.Remove(etcd)
	}

	if sv.leaderIP != leaderIP || sv.IP != IP {
		sv.Remove(kubelet)
	}

	sv.run(etcd, "--discovery="+etcdToken, "--proxy=on",
		"--heartbeat-interval="+etcdHeartbeatInterval,
		"--election-timeout="+etcdElectionTimeout)

	sv.run(ovsdb)
	sv.run(ovsvswitchd)

	if leaderIP == "" || IP == "" {
		return
	}

	sv.run(kubelet, "/usr/bin/boot-worker", IP, leaderIP)

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

	/* Create the di logical switch if it doesn't exist. */
	if err := sv.dk.CreateLSwitch("di"); err != nil {
		log.Warning("Failed to create LSwitch: %s", err)
	}
}

func (sv *supervisor) updateMaster(IP, etcdToken string, leader bool) {
	if sv.IP != IP || sv.etcdToken != etcdToken {
		sv.Remove(etcd)
	}

	if sv.IP != IP {
		sv.Remove(kubelet)
	}

	if IP == "" || etcdToken == "" {
		return
	}

	sv.run(etcd, fmt.Sprintf("--name=master-%s", IP),
		fmt.Sprintf("--discovery=%s", etcdToken),
		fmt.Sprintf("--advertise-client-urls=http://%s:2379", IP),
		fmt.Sprintf("--listen-peer-urls=http://%s:2380", IP),
		fmt.Sprintf("--initial-advertise-peer-urls=http://%s:2380", IP),
		"--listen-client-urls=http://0.0.0.0:2379",
		"--heartbeat-interval="+etcdHeartbeatInterval,
		"--election-timeout="+etcdElectionTimeout)
	sv.run(ovsdb)
	sv.run(kubelet, "/usr/bin/boot-master", IP)

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
	case kubelet:
		ro.Binds = []string{
			"/proc:/hostproc:ro",
			"/var/run:/var/run:rw",
			"/var/lib/docker:/var/lib/docker:rw",
			"/etc/docker:/etc/docker:rw",
			"/dev:/dev:rw",
			"/sys:/sys:ro",
		}
		ro.Privileged = true
		ro.PidMode = "host" // Use PID=host per the kubernetes documentation.
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
