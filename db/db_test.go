package db

import (
	"fmt"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/davecgh/go-spew/spew"
)

func TestMachine(t *testing.T) {
	conn := New()

	var m Machine
	err := conn.Transact(func(db Database) error {
		m = db.InsertMachine()
		return nil
	})
	if err != nil {
		t.FailNow()
	}

	if m.ID != 1 || m.ClusterID != 0 || m.Role != None || m.CloudID != "" ||
		m.PublicIP != "" || m.PrivateIP != "" {
		t.Errorf("Invalid Machine: %s", spew.Sdump(m))
		return
	}

	old := m

	m.ClusterID = 2
	m.Role = Worker
	m.CloudID = "something"
	m.PublicIP = "1.2.3.4"
	m.PrivateIP = "5.6.7.8"

	err = conn.Transact(func(db Database) error {
		if err := SelectMachineCheck(db, nil, []Machine{old}); err != nil {
			return err
		}

		db.Commit(m)

		if err := SelectMachineCheck(db, nil, []Machine{m}); err != nil {
			return err
		}

		db.Remove(m)

		if err := SelectMachineCheck(db, nil, []Machine{}); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		t.Error(err.Error())
		return
	}
}

func TestMachineSelect(t *testing.T) {
	conn := New()

	var machines []Machine
	err := conn.Transact(func(db Database) error {
		for i := 0; i < 4; i++ {
			m := db.InsertMachine()
			m.ClusterID = i
			db.Commit(m)
			machines = append(machines, m)
		}
		return nil
	})

	err = conn.Transact(func(db Database) error {
		err := SelectMachineCheck(db, func(m Machine) bool {
			return m.ClusterID%2 == 0
		}, []Machine{machines[0], machines[2]})
		if err != nil {
			return err
		}

		err = SelectMachineCheck(db, func(m Machine) bool {
			return m.ClusterID%2 == 1
		}, []Machine{machines[1], machines[3]})
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		t.Error(err.Error())
		return
	}
}

func TestCluster(t *testing.T) {
	conn := New()

	err := conn.Transact(func(db Database) error {
		c := db.InsertCluster()
		c.RedCount = 1
		db.Commit(c)

		c = db.InsertCluster()
		c.RedCount = 2
		db.Commit(c)

		clusters := db.SelectFromCluster(nil)
		if len(clusters) != 2 {
			return fmt.Errorf("Expected 2 clusters, found %s",
				spew.Sdump(clusters))
		}

		return nil
	})
	if err != nil {
		t.Error(err.Error())
		return
	}

	err = conn.Transact(func(db Database) error {
		clusters := db.SelectFromCluster(func(c Cluster) bool {
			return c.RedCount == 1
		})
		if len(clusters) != 1 || clusters[0].RedCount != 1 {
			return fmt.Errorf("Incorrect clusters: %s", spew.Sdump(clusters))
		}

		clusters = db.SelectFromCluster(func(c Cluster) bool {
			return c.RedCount == 2
		})
		if len(clusters) != 1 || clusters[0].RedCount != 2 {
			return fmt.Errorf("Incorrect clusters: %s", spew.Sdump(clusters))
		}
		return nil
	})
	if err != nil {
		t.Error(err.Error())
		return
	}

	err = conn.Transact(func(db Database) error {
		clusters := db.SelectFromCluster(nil)
		for _, clst := range clusters {
			db.Remove(clst)
		}

		clusters = db.SelectFromCluster(nil)
		if len(clusters) > 0 {
			return fmt.Errorf("Incorrect clusters: %s", spew.Sdump(clusters))
		}

		return nil
	})
	if err != nil {
		t.Error(err.Error())
		return
	}
}

func TestTrigger(t *testing.T) {
	conn := New()

	mt := conn.Trigger(MachineTable)
	mt2 := conn.Trigger(MachineTable)
	ct := conn.Trigger(ClusterTable)
	ct2 := conn.Trigger(ClusterTable)

	triggerNoRecv(t, mt)
	triggerNoRecv(t, mt2)
	triggerNoRecv(t, ct)
	triggerNoRecv(t, ct2)

	err := conn.Transact(func(db Database) error {
		db.InsertMachine()
		return nil
	})
	if err != nil {
		t.Fail()
		return
	}

	triggerRecv(t, mt)
	triggerRecv(t, mt2)
	triggerNoRecv(t, ct)
	triggerNoRecv(t, ct2)

	mt2.Stop()
	err = conn.Transact(func(db Database) error {
		db.InsertMachine()
		return nil
	})
	if err != nil {
		t.Fail()
		return
	}
	triggerRecv(t, mt)

	mt.Stop()
	ct.Stop()
	ct2.Stop()

	fast := conn.TriggerTick(1, MachineTable)
	triggerRecv(t, fast)
	triggerRecv(t, fast)
	triggerRecv(t, fast)
}

func triggerRecv(t *testing.T, trig Trigger) {
	select {
	case <-trig.C:
	case <-time.Tick(5 * time.Second):
		t.Error("Expected Receive")
	}
}

func triggerNoRecv(t *testing.T, trig Trigger) {
	select {
	case <-trig.C:
		t.Error("Unexpected Receive")
	case <-time.Tick(25 * time.Millisecond):
	}
}

func SelectMachineCheck(db Database, do func(Machine) bool, expected []Machine) error {
	query := db.SelectFromMachine(do)
	sort.Sort(mSort(expected))
	sort.Sort(mSort(query))
	if !reflect.DeepEqual(expected, query) {
		return fmt.Errorf("Unexpect query result: %s\nExpected %s",
			spew.Sdump(query), spew.Sdump(expected))
	}

	return nil
}

type mSort []Machine

func (machines mSort) sort() {
	sort.Stable(machines)
}

func (machines mSort) Len() int {
	return len(machines)
}

func (machines mSort) Swap(i, j int) {
	machines[i], machines[j] = machines[j], machines[i]
}

func (machines mSort) Less(i, j int) bool {
	return machines[i].ID < machines[j].ID
}
