package db

import (
	"reflect"
)

type TableType string

/* Used by the global controller. */
var ClusterTable TableType = TableType(reflect.TypeOf(Cluster{}).String())
var MachineTable TableType = TableType(reflect.TypeOf(Machine{}).String())

/* Used by the minions. */
var ContainerTable TableType = TableType(reflect.TypeOf(Container{}).String())
var MinionTable TableType = TableType(reflect.TypeOf(Minion{}).String())
var ConnectionTable TableType = TableType(reflect.TypeOf(Connection{}).String())
var LabelTable TableType = TableType(reflect.TypeOf(Label{}).String())
var EtcdTable TableType = TableType(reflect.TypeOf(Etcd{}).String())

var allTables []TableType = []TableType{ClusterTable, MachineTable, ContainerTable,
	MinionTable, ConnectionTable, LabelTable, EtcdTable}

type table struct {
	rows map[int]row

	triggers map[Trigger]struct{}
	trigSeq  int
	seq      int
}

func newTable() *table {
	return &table{
		rows:     make(map[int]row),
		triggers: make(map[Trigger]struct{}),
	}
}

func (t *table) alert() {
	if t.seq == t.trigSeq {
		return
	}
	t.trigSeq = t.seq

	for trigger := range t.triggers {
		select {
		case <-trigger.stop:
			delete(t.triggers, trigger)
		default:
		}

		select {
		case trigger.C <- struct{}{}:
		default:
		}
	}
}
