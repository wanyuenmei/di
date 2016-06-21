package network

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"sort"
	"time"

	"github.com/NetSys/quilt/db"
	"github.com/NetSys/quilt/join"
	"github.com/NetSys/quilt/minion/consensus"

	log "github.com/Sirupsen/logrus"
)

const labelDir = "/minion/labels"
const containerDir = "/minion/containers"

// We store rand.Uint32() in a variable so it's easily mocked out by the unit tests.
// Nondeterminism is hard to test.
var rand32 = rand.Uint32

// A directory containers the first and seceond level of a Tree requested from the
// consensus store.
type directory map[string]map[string]string

// wakeChan collapses the various channels these functions wait on into a single
// channel. Multiple redundant pings will be coalesced into a single message.
func wakeChan(conn db.Conn, store consensus.Store) chan struct{} {
	labelWatch := store.Watch(labelDir, 1*time.Second)
	containerWatch := store.Watch(labelDir, 1*time.Second)
	trigg := conn.TriggerTick(30, db.MinionTable, db.ContainerTable, db.LabelTable,
		db.EtcdTable).C

	c := make(chan struct{}, 1)
	go func() {
		for {
			select {
			case <-labelWatch:
			case <-containerWatch:
			case <-trigg:
			}

			select {
			case c <- struct{}{}:
			default: // There's a notification in queue no need for another.
			}
		}
	}()

	return c
}

func readStoreRun(conn db.Conn, store consensus.Store) {
	// If the directories don't exist, create them so we may watch them.  If they
	// exist already these will return an error that we won't log, but that's ok
	// cause the loop will error too.
	store.Mkdir(labelDir)
	store.Mkdir(containerDir)

	for range wakeChan(conn, store) {
		labelDir, err := getDirectory(store, labelDir)
		containerDir, err2 := getDirectory(store, containerDir)
		if err2 != nil {
			err = err2
		}

		if err != nil {
			log.WithError(err).Warn("Failed to read from cluster store.")
			continue
		}

		conn.Transact(func(view db.Database) error {
			readContainerTransact(view, containerDir)
			readLabelTransact(view, labelDir)
			return nil
		})
	}
}

func readContainerTransact(view db.Database, dir directory) {
	minions := view.SelectFromMinion(nil)
	worker := len(minions) == 1 && minions[0].Role == db.Worker

	for _, container := range view.SelectFromContainer(nil) {
		container.IP = ""
		var labels []string
		if children, ok := dir[container.SchedID]; ok {
			json.Unmarshal([]byte(children["Labels"]), &labels)

			container.IP = children["IP"]
			ip := net.ParseIP(container.IP).To4()
			if ip != nil {
				container.Mac = fmt.Sprintf("02:00:%02x:%02x:%02x:%02x",
					ip[0], ip[1], ip[2], ip[3])
			}
		}

		if worker {
			// Masters get their labels from the policy, workers from the
			// consensus store.
			container.Labels = labels
		}

		view.Commit(container)
	}
}

func readLabelTransact(view db.Database, dir directory) {
	lKey := func(val interface{}) interface{} {
		return val.(db.Label).Label
	}
	pairs, dbls, dirKeys := join.HashJoin(db.LabelSlice(view.SelectFromLabel(nil)),
		StringSlice(dir.keys()), lKey, nil)

	for _, dbl := range dbls {
		view.Remove(dbl.(db.Label))
	}

	for _, key := range dirKeys {
		pairs = append(pairs, join.Pair{L: view.InsertLabel(), R: key})
	}

	for _, pair := range pairs {
		dbl := pair.L.(db.Label)
		dbl.Label = pair.R.(string)
		dbl.IP = dir[dbl.Label]["IP"]
		_, dbl.MultiHost = dir[dbl.Label]["MultiHost"]
		view.Commit(dbl)
	}
}

func writeStoreRun(conn db.Conn, store consensus.Store) {
	for range wakeChan(conn, store) {
		leader := false
		var containers []db.Container
		conn.Transact(func(view db.Database) error {
			etcds := view.SelectFromEtcd(nil)
			leader = len(etcds) == 1 && etcds[0].Leader
			containers = view.SelectFromContainer(nil)
			return nil
		})

		if !leader {
			continue
		}

		if err := writeStoreContainers(store, containers); err != nil {
			log.WithError(err).Warning("Failed to update containers in ETCD")
		}

		writeStoreLabels(store, containers)
	}
}

func writeStoreContainers(store consensus.Store, containers []db.Container) error {
	var ids []string
	for _, container := range containers {
		if container.SchedID != "" {
			ids = append(ids, container.SchedID)
		}
	}

	store.Mkdir(containerDir)
	dir, err := getDirectory(store, containerDir)
	if err != nil {
		return err
	}

	syncDir(store, dir, containerDir, ids)
	syncIPs(store, dir, containerDir, net.IPv4(10, 0, 0, 0))
	syncLabels(store, dir, containerDir, containers)

	return nil
}

func writeStoreLabels(store consensus.Store, containers []db.Container) error {
	store.Mkdir(labelDir)
	dir, err := getDirectory(store, labelDir)
	if err != nil {
		return err
	}

	var ids []string
	for _, c := range containers {
		for _, l := range c.Labels {
			ids = append(ids, l)
		}
	}

	syncDir(store, dir, labelDir, ids)

	// Labels that point to a single container don't need IPs separate from their
	// constituent container IP.  Thus the following code marks those labels, by the
	// absence of the `MultiHost` file in the consensus store, and sets their IP to
	// whatever is found in the container table.

	// Map from each label to the containers that implement it.
	labelMap := map[string][]db.Container{}
	for _, dbc := range containers {
		for _, label := range dbc.Labels {
			labelMap[label] = append(labelMap[label], dbc)
		}
	}

	// Mark labels as MultiHost if they have more than one container.
	for label, dbcs := range labelMap {
		isMH := len(dbcs) > 1
		if _, ok := dir[label]["MultiHost"]; ok == isMH {
			continue
		}

		var err error
		path := fmt.Sprintf("%s/%s/MultiHost", labelDir, label)
		if isMH {
			dir[label]["MultiHost"] = "true"
			err = store.Set(path, "true")
		} else {
			delete(dir[label], "MultiHost")
			err = store.Delete(path)
		}

		if err != nil {
			log.WithFields(log.Fields{
				"label": label,
				"error": err,
			}).Warn("Failed to change MultiHost key.")
		}
	}

	// Set the IPs of the non-MultiHost containers.
	for label, dbcs := range labelMap {
		if len(dbcs) != 1 {
			continue
		}

		dbc := dbcs[0]
		if dbc.IP == "" || dir[label]["IP"] == dbc.IP {
			continue
		}

		path := fmt.Sprintf("%s/%s/IP", labelDir, label)
		if err := store.Set(path, dbc.IP); err != nil {
			log.WithFields(log.Fields{
				"label": label,
				"IP":    dbc.IP,
				"error": err,
			}).Warn("Consensus store failed set label IP.")
		}
	}

	// Remove the non-MultiHost containers from `dir` to avoid confusing syncIPs.
	for label, mp := range dir {
		if _, ok := mp["MultiHost"]; !ok {
			delete(dir, label)
		}
	}

	syncIPs(store, dir, labelDir, net.IPv4(10, 1, 0, 0))
	return nil
}

func syncDir(store consensus.Store, dir directory, path string, idsArg []string) {
	_, dirKeys, ids := join.HashJoin(StringSlice(dir.keys()), StringSlice(idsArg), nil,
		nil)

	var etcdLog string
	for _, dirKey := range dirKeys {
		id := dirKey.(string)
		keyPath := fmt.Sprintf("%s/%s", path, id)
		err := store.Delete(keyPath)
		if err != nil {
			etcdLog = fmt.Sprintf("Failed to delete %s: %s", keyPath, err)
		}
		delete(dir, id)
	}

	for _, idElem := range ids {
		id := idElem.(string)
		if _, ok := dir[id]; ok {
			continue
		}

		key := fmt.Sprintf("%s/%s", path, id)
		if err := store.Mkdir(key); err != nil {
			etcdLog = fmt.Sprintf("Failed to create dir %s: %s", key, err)
			continue
		}
		dir[id] = map[string]string{}
	}

	// Etcd failure leads to a bunch of useless errors.  Therefore we only log once.
	if etcdLog != "" {
		log.Error(etcdLog)
	}
}

// syncIPs() takes a directory and creates an IP node for every entry that's missing
// one.
func syncIPs(store consensus.Store, dir directory, path string, prefixIP net.IP) {
	prefix := binary.BigEndian.Uint32(prefixIP.To4())
	mask := uint32(0xffff0000)

	var unassigned []string
	ipSet := map[uint32]struct{}{}
	for k, m := range dir {
		ip := parseIP(m["IP"], prefix, mask)
		if ip != 0 {
			ipSet[ip] = struct{}{}
		} else {
			unassigned = append(unassigned, k)
		}
	}

	// Don't assign the IP of the default gateway
	ipSet[parseIP(gatewayIP, prefix, mask)] = struct{}{}
	var etcdLog string
	for _, k := range unassigned {
		ip32 := randomIP(ipSet, prefix, mask)
		ipPath := fmt.Sprintf("%s/%s/IP", path, k)
		if ip32 == 0 {
			log.Errorf("Failed to allocate IP for %s.", k)
			store.Delete(ipPath)
			delete(dir[k], "IP")
			continue
		}

		b := make([]byte, 4)
		binary.BigEndian.PutUint32(b, ip32)

		ipStr := net.IP(b).String()
		if err := store.Set(ipPath, ipStr); err != nil {
			etcdLog = fmt.Sprintf("Failed to set key %s: %s", ipPath, err)
			continue
		}

		dir[k]["IP"] = ipStr
		ipSet[ip32] = struct{}{}
	}

	// Etcd failure leads to a bunch of useless errors.  Therefore we only log once.
	if etcdLog != "" {
		log.Error(etcdLog)
	}
}

func syncLabels(store consensus.Store, dir directory, path string,
	containers []db.Container) {

	idLabelMap := map[string][]string{}
	for _, container := range containers {
		if container.SchedID != "" {
			idLabelMap[container.SchedID] = container.Labels
		}
	}

	for id, children := range dir {
		labels := idLabelMap[id]
		if labels == nil {
			labels = []string{}
		}
		sort.Sort(sort.StringSlice(labels))

		jsByte, err := json.Marshal(labels)
		if err != nil {
			panic("Not Reached")
		}
		js := string(jsByte)

		if js == children["Labels"] {
			continue
		}

		key := fmt.Sprintf("%s/%s/Labels", path, id)
		if err := store.Set(key, js); err != nil {
			log.WithField("path", path).Error("Failed to set label key.")
			continue
		}
		dir[id]["Labels"] = js
	}
}

func getDirectory(store consensus.Store, path string) (directory, error) {
	tree, err := store.GetTree(path)
	if err != nil {
		return nil, err
	}

	dir := make(directory)
	for _, l1 := range tree.Children {
		childMap := make(map[string]string)
		for _, l2 := range l1.Children {
			childMap[l2.Key] = l2.Value
		}
		dir[l1.Key] = childMap
	}

	return dir, nil
}

func parseIP(ipStr string, prefix, mask uint32) uint32 {
	ip := net.ParseIP(ipStr).To4()
	if ip == nil {
		return 0
	}

	ip32 := binary.BigEndian.Uint32(ip)
	if ip32&mask != prefix {
		return 0
	}

	return ip32
}

// Choose a random IP address in prefix/mask that isn't in 'conflicts'.
// Returns 0 on failure.
func randomIP(conflicts map[uint32]struct{}, prefix, mask uint32) uint32 {
	for i := 0; i < 256; i++ {
		ip32 := (rand32() & ^mask) | (prefix & mask)
		if _, ok := conflicts[ip32]; !ok {
			return ip32
		}
	}

	return 0
}

func (dir directory) keys() []string {
	var keys []string
	for key := range dir {
		keys = append(keys, key)
	}
	return keys
}

// StringSlice is an alias for []string to allow for joins
type StringSlice []string

// Get returns the value contained at the given index
func (ss StringSlice) Get(ii int) interface{} {
	return ss[ii]
}

// Len returns the number of items in the slice
func (ss StringSlice) Len() int {
	return len(ss)
}
