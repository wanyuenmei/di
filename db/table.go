package db

type TableType int

const (
	/* Used by the global controller. */
	ClusterTable TableType = iota
	MachineTable

	/* Used by the minions. */
	ContainerTable
	MinionTable
	ConnectionTable
	LabelTable
	EtcdTable
)

var allTables = map[TableType]string{
	ClusterTable:    "Cluster",
	MachineTable:    "Machine",
	ContainerTable:  "Container",
	MinionTable:     "Minion",
	EtcdTable:       "Etcd",
	ConnectionTable: "Connection",
	LabelTable:      "Label",
}

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

func (t TableType) String() string {
	return allTables[t]
}
