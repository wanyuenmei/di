package supervisor

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/NetSys/quilt/db"
	"github.com/NetSys/quilt/join"
	"github.com/NetSys/quilt/minion/docker"

	log "github.com/Sirupsen/logrus"
)

const (
	// Etcd is the name etcd cluster store container.
	Etcd = "etcd"

	// Ovncontroller is the name of the OVN controller container.
	Ovncontroller = "ovn-controller"

	// Ovnnorthd is the name of the OVN northd container.
	Ovnnorthd = "ovn-northd"

	// Ovsdb is the name of the OVSDB container.
	Ovsdb = "ovsdb-server"

	// Ovsvswitchd is the name of the ovs-vswitchd container.
	Ovsvswitchd = "ovs-vswitchd"

	// Swarm is the name of the docker swarm.
	Swarm = "swarm"

	// QuiltTag is the name of the container used to tag the machine for placement.
	QuiltTag = "quilt-tag"
)

const ovsImage = "quilt/ovs"

var images = map[string]string{
	Etcd:          "quay.io/coreos/etcd:v2.3.6",
	Ovncontroller: ovsImage,
	Ovnnorthd:     ovsImage,
	Ovsdb:         ovsImage,
	Ovsvswitchd:   ovsImage,
	Swarm:         "swarm:1.2.3",
	QuiltTag:      "google/pause",
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
	provider string
	region   string
	size     string
}

// Run blocks implementing the supervisor module.
func Run(conn db.Conn, dk docker.Client) {
	sv := supervisor{conn: conn, dk: dk}
	go sv.runSystem()
	sv.runApp()
}

// Synchronize locally running "application" containers with the database.
func (sv *supervisor) runApp() {
	for range sv.conn.TriggerTick(10, db.MinionTable, db.ContainerTable).C {
		minions := sv.conn.SelectFromMinion(nil)
		if len(minions) != 1 || minions[0].Role != db.Worker {
			continue
		}

		if err := delStopped(sv.dk); err != nil {
			log.WithError(err).Error("Failed to clean up stopped containers")
		}

		dkcs, err := sv.dk.List(map[string][]string{
			"label": {docker.SchedulerLabelPair},
		})
		if err != nil {
			log.WithError(err).Error("Failed to list local containers.")
			continue
		}

		sv.conn.Transact(func(view db.Database) error {
			sv.runAppTransact(view, dkcs)
			return nil
		})
	}
}

// Delete stopped containers
//
// We do this because Docker Swarm will account for stopped containers
// when using its affinity filter, where our semantics don't consider
// stopped containers in its scheduling decisions.
func delStopped(dk docker.Client) error {
	containers, err := dk.List(map[string][]string{"status": {"exited"}})
	if err != nil {
		return fmt.Errorf("error listing stopped containers: %s", err)
	}
	for _, dkc := range containers {
		// Stopped containers show up with a "/" in front of the name
		name := dkc.Name[1:]
		if err := dk.Remove(name); err != nil {
			log.WithFields(log.Fields{
				"name": name,
				"err":  err,
			}).Error("error removing container")
			continue
		}
	}
	return nil
}

func (sv *supervisor) runAppTransact(view db.Database,
	dkcsArgs []docker.Container) []string {

	var tearDowns []string

	dbKey := func(val interface{}) interface{} {
		return val.(db.Container).SchedID
	}
	dkKey := func(val interface{}) interface{} {
		return val.(docker.Container).ID
	}

	pairs, dbcs, dkcs := join.HashJoin(db.ContainerSlice(view.SelectFromContainer(nil)),
		docker.ContainerSlice(dkcsArgs), dbKey, dkKey)

	for _, iface := range dbcs {
		dbc := iface.(db.Container)

		tearDowns = append(tearDowns, dbc.SchedID)
		view.Remove(dbc)
	}

	for _, dkc := range dkcs {
		pairs = append(pairs, join.Pair{L: view.InsertContainer(), R: dkc})
	}

	for _, pair := range pairs {
		dbc := pair.L.(db.Container)
		dkc := pair.R.(docker.Container)

		dbc.SchedID = dkc.ID
		dbc.Pid = dkc.Pid
		dbc.Image = dkc.Image
		dbc.Command = append([]string{dkc.Path}, dkc.Args...)
		view.Commit(dbc)
	}

	return tearDowns
}

// Manage system infrstracture containers that support the application.
func (sv *supervisor) runSystem() {
	imageSet := map[string]struct{}{}
	for _, image := range images {
		imageSet[image] = struct{}{}
	}

	for image := range imageSet {
		go sv.dk.Pull(image)
	}

	for range sv.conn.Trigger(db.MinionTable, db.EtcdTable).C {
		sv.runSystemOnce()
	}
}

func (sv *supervisor) runSystemOnce() {
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
		sv.leader == etcdRow.Leader &&
		sv.provider == minion.Provider &&
		sv.region == minion.Region &&
		sv.size == minion.Size {
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
	sv.provider = minion.Provider
	sv.region = minion.Region
	sv.size = minion.Size
}

func (sv *supervisor) tagWorker(provider, region, size string) {
	if sv.provider != provider || sv.region != region || sv.size != size {
		sv.Remove(QuiltTag)
	}
	tags := map[string]string{
		docker.SystemLabel("provider"): provider,
		docker.SystemLabel("region"):   region,
		docker.SystemLabel("size"):     size,
	}

	ro := docker.RunOptions{
		Name:        QuiltTag,
		Image:       images[QuiltTag],
		Labels:      tags,
		NetworkMode: "host",
	}
	if err := sv.dk.Run(ro); err != nil {
		log.WithError(err).Warn("Failed to tag minion.")
	}
}

func (sv *supervisor) updateWorker(IP string, leaderIP string, etcdIPs []string) {
	if !reflect.DeepEqual(sv.etcdIPs, etcdIPs) {
		sv.Remove(Etcd)
	}

	if sv.leaderIP != leaderIP || sv.IP != IP {
		sv.Remove(Swarm)
	}

	sv.run(Etcd, fmt.Sprintf("--initial-cluster=%s", initialClusterString(etcdIPs)),
		"--heartbeat-interval="+etcdHeartbeatInterval,
		"--election-timeout="+etcdElectionTimeout,
		"--proxy=on")

	sv.run(Ovsdb, "ovsdb-server")
	sv.run(Ovsvswitchd, "ovs-vswitchd")

	if leaderIP == "" || IP == "" {
		return
	}

	sv.run(Swarm, "join", fmt.Sprintf("--addr=%s:2375", IP), "etcd://127.0.0.1:2379")

	minions := sv.conn.SelectFromMinion(nil)
	if len(minions) != 1 {
		return
	}
	minion := minions[0]

	err := sv.dk.Exec(Ovsvswitchd, "ovs-vsctl", "set", "Open_vSwitch", ".",
		fmt.Sprintf("external_ids:ovn-remote=\"tcp:%s:6640\"", leaderIP),
		fmt.Sprintf("external_ids:ovn-encap-ip=%s", IP),
		"external_ids:ovn-encap-type=\"geneve\"",
		fmt.Sprintf("external_ids:api_server=\"http://%s:9000\"", leaderIP),
		fmt.Sprintf("external_ids:system-id=\"%s\"", IP),
		"--", "add-br", "quilt-int",
		"--", "set", "bridge", "quilt-int", "fail_mode=secure")
	if err != nil {
		log.WithError(err).Warnf("Failed to exec in %s.", Ovsvswitchd)
	}

	/* The ovn controller doesn't support reconfiguring ovn-remote mid-run.
	 * So, we need to restart the container when the leader changes. */
	sv.Remove(Ovncontroller)
	sv.run(Ovncontroller, "ovn-controller")

	sv.tagWorker(minion.Provider, minion.Region, minion.Size)
}

func (sv *supervisor) updateMaster(IP string, etcdIPs []string, leader bool) {
	if sv.IP != IP || !reflect.DeepEqual(sv.etcdIPs, etcdIPs) {
		sv.Remove(Etcd)
	}

	if sv.IP != IP {
		sv.Remove(Swarm)
	}

	if IP == "" || len(etcdIPs) == 0 {
		return
	}

	sv.run(Etcd, fmt.Sprintf("--name=master-%s", IP),
		fmt.Sprintf("--initial-cluster=%s", initialClusterString(etcdIPs)),
		fmt.Sprintf("--advertise-client-urls=http://%s:2379", IP),
		fmt.Sprintf("--listen-peer-urls=http://%s:2380", IP),
		fmt.Sprintf("--initial-advertise-peer-urls=http://%s:2380", IP),
		"--listen-client-urls=http://0.0.0.0:2379",
		"--heartbeat-interval="+etcdHeartbeatInterval,
		"--initial-cluster-state=new",
		"--election-timeout="+etcdElectionTimeout)
	sv.run(Ovsdb, "ovsdb-server")

	swarmAddr := IP + ":2377"
	sv.run(Swarm, "manage", "--replication", "--addr="+swarmAddr,
		"--host="+swarmAddr, "etcd://127.0.0.1:2379")

	if leader {
		/* XXX: If we fail to boot ovn-northd, we should give up
		* our leadership somehow.  This ties into the general
		* problem of monitoring health. */
		sv.run(Ovnnorthd, "ovn-northd")
	} else {
		sv.Remove(Ovnnorthd)
	}
}

func (sv *supervisor) run(name string, args ...string) {
	isRunning, err := sv.dk.IsRunning(name)
	if err != nil {
		log.WithError(err).Warnf("could not check running status of %s.", name)
		return
	}
	if isRunning {
		return
	}

	ro := docker.RunOptions{
		Name:        name,
		Image:       images[name],
		Args:        args,
		NetworkMode: "host",
	}

	switch name {
	case Ovsvswitchd:
		ro.Privileged = true
		ro.VolumesFrom = []string{Ovsdb}
	case Ovnnorthd:
		ro.VolumesFrom = []string{Ovsdb}
	case Ovncontroller:
		ro.VolumesFrom = []string{Ovsdb}
	}

	if err := sv.dk.Run(ro); err != nil {
		log.WithError(err).Warnf("Failed to run %s.", name)
	}
}

func (sv *supervisor) Remove(name string) {
	if err := sv.dk.Remove(name); err != nil {
		log.WithError(err).Warnf("Failed to remove %s.")
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
		initialCluster = append(initialCluster,
			fmt.Sprintf("%s=http://%s:2380", nodeName(ip), ip))
	}
	return strings.Join(initialCluster, ",")
}

func nodeName(IP string) string {
	return fmt.Sprintf("master-%s", IP)
}
