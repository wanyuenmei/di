package network

import (
	"encoding/binary"
	"encoding/json"
	"math/rand"
	"net"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/NetSys/di/db"
	"github.com/NetSys/di/minion/consensus"
	"github.com/op/go-logging"
)

var log = logging.MustGetLogger("network")

const labelDir = "/minion/labels"

type labelData struct {
	IP       string
	SchedIDs []string
}

func readLabels(conn db.Conn, store consensus.Store) {
	watch := store.Watch(labelDir, 5*time.Second)
	tick := time.Tick(10 * time.Second)
	for {
		readLabelsRun(conn, store)
		select {
		case <-watch:
		case <-tick:
		}
	}
}

func readLabelsRun(conn db.Conn, store consensus.Store) {
	etcdLabels, err := getLabels(store)
	if err != nil {
		return
	}

	conn.Transact(func(view db.Database) error {
		dbLabels := view.SelectFromLabel(nil)
		for _, l := range dbLabels {
			el, ok := etcdLabels[l.Label]
			if !ok {
				view.Remove(l)
				continue
			}

			l.IP = el.IP
			l.SchedIDs = el.SchedIDs
			view.Commit(l)
			delete(etcdLabels, l.Label)
		}

		for key, ld := range etcdLabels {
			label := view.InsertLabel()
			label.Label = key
			label.IP = ld.IP
			label.SchedIDs = ld.SchedIDs
			view.Commit(label)
		}

		return nil
	})
}

func writeLabels(conn db.Conn, store consensus.Store) {
	trigg := conn.TriggerTick(60, db.MinionTable, db.ContainerTable).C
	watch := store.Watch(labelDir, 5*time.Second)
	for {
		select {
		case <-trigg:
		case <-watch:
		}

		minions := conn.SelectFromMinion(nil)
		if len(minions) != 1 || !minions[0].Leader {
			continue
		}

		store.Mkdir(labelDir)

		kvLabels, err := getLabels(store)
		if err != nil {
			log.Warning(err.Error())
			continue
		}

		remove, change := diffLabels(kvLabels, conn.SelectFromContainer(nil))

		for _, r := range remove {
			if err = store.Delete(labelDir + "/" + r); err != nil {
				log.Warning("Failed to remove %s: %s", r, err)
			}
		}

		for l, ld := range change {
			json, err := json.Marshal(*ld)
			if err != nil {
				panic("Not Reached")
			}

			err = store.Set(labelDir+"/"+l, string(json))
			if err != nil {
				log.Warning("Failed to set label: %s", l)
			}
		}
	}
}

func diffLabels(kvLabels map[string]*labelData,
	containers []db.Container) (remove []string, change map[string]*labelData) {

	change = make(map[string]*labelData)
	for _, c := range containers {
		if c.SchedID == "" {
			continue
		}

		for _, l := range c.Labels {
			ld, ok := change[l]
			if !ok {
				change[l] = &labelData{}
				ld = change[l]
			}

			ld.SchedIDs = append(ld.SchedIDs, c.SchedID)
		}
	}

	for l := range kvLabels {
		if _, ok := change[l]; !ok {
			remove = append(remove, l)
			delete(kvLabels, l)
		}
	}

	prefix := binary.BigEndian.Uint32(net.IPv4(10, 1, 0, 0).To4())
	mask := uint32(0xffff0000)

	// Set the IPs in 'change' with what's found in etcd, and initialize ipSet with
	// the ips that have been allocated already.
	ipSet := map[uint32]struct{}{}
	for l, ld := range kvLabels {
		change[l].IP = ""

		ip := net.ParseIP(ld.IP).To4()
		if ip == nil {
			continue
		}

		ip32 := binary.BigEndian.Uint32(ip)
		if ip32&mask != prefix {
			continue
		}

		ipSet[ip32] = struct{}{}
		change[l].IP = ip.String()
	}

	// Set IPs for elements of 'change' that don't have one.
	for l, ld := range change {
		if ld.IP != "" {
			continue
		}

		var ip32 uint32
		for i := 0; i < 256; i++ {
			ip32 = (rand.Uint32() & ^mask) | prefix
			if _, ok := ipSet[ip32]; !ok {
				break
			}
			ip32 = 0
		}

		if ip32 == 0 {
			log.Warning("Failed to allocate IP for: %s", l)
			remove = append(remove, l)
			delete(change, l)
			continue
		}

		b := make([]byte, 4)
		binary.BigEndian.PutUint32(b, ip32)
		ld.IP = net.IP(b).String()
	}

	// Remove elements from 'change' that dont need updating.
	for l, ld := range change {
		sort.Sort(sort.StringSlice(ld.SchedIDs))
		ldAct := kvLabels[l]
		if ldAct != nil && reflect.DeepEqual(ld, ldAct) {
			delete(change, l)
		}
	}

	return remove, change
}

func getLabels(store consensus.Store) (map[string]*labelData, error) {
	labels, err := store.GetDir(labelDir)
	if err != nil {
		return nil, err
	}

	kvLabels := make(map[string]*labelData)

	// Initialize 'kvLabels' with the set of labels in etcd.
	for k, v := range labels {
		ld := &labelData{}
		err := json.Unmarshal([]byte(v), ld)
		if err != nil {
			ld = &labelData{}
			log.Warning("Failed to parse label: %s", k)
		}
		kvLabels[strings.TrimPrefix(k, labelDir+"/")] = ld
	}

	return kvLabels, nil
}
