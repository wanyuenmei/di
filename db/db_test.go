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

	if m.ID != 1 || m.Role != None || m.CloudID != "" || m.PublicIP != "" ||
		m.PrivateIP != "" {
		t.Errorf("Invalid Machine: %s", spew.Sdump(m))
		return
	}

	old := m

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
	regions := []string{"here", "there", "anywhere", "everywhere"}

	var machines []Machine
	conn.Transact(func(db Database) error {
		for i := 0; i < 4; i++ {
			m := db.InsertMachine()
			m.Region = regions[i]
			db.Commit(m)
			machines = append(machines, m)
		}
		return nil
	})

	err := conn.Transact(func(db Database) error {
		err := SelectMachineCheck(db, func(m Machine) bool {
			return m.Region == "there"
		}, []Machine{machines[1]})
		if err != nil {
			return err
		}

		err = SelectMachineCheck(db, func(m Machine) bool {
			return m.Region != "there"
		}, []Machine{machines[0], machines[2], machines[3]})
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
		return fmt.Errorf("unexpected query result: %s\nExpected %s",
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
