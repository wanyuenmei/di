package db

type TableType int

const (
	/* Used by the global controller. */
	ClusterTable TableType = iota
	MachineTable

	/* Used by the minions. */
	ContainerTable
	MinionTable
)

type table struct {
	idAlloc int
	rows    map[int]row

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

func (t *table) insert(r row, id int) {
	t.seq++
	t.rows[id] = r
}

func (t *table) write(r row, id int) {
	if !t.rows[id].equal(r) {
		t.seq++
		t.rows[id] = r
	}
}

func (t *table) remove(id int) {
	t.seq++
	delete(t.rows, id)
}

func (t *table) nextID() int {
	t.idAlloc += 1
	return t.idAlloc
}
