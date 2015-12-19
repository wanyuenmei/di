package db

import "time"

// The Database is the central storage location for all state in the system.  The policy
// engine populates the database with a preferred state of the world, while various
// modules flesh out that policy with actual implementation details.
type Database struct {
	machine table
	cluster table
	idAlloc int
}

// A Trigger sends notifications when anything in their corresponding table changes.
type Trigger struct {
	C    chan struct{} // The channel on which notifications are delivered.
	stop chan struct{}
}

type row interface {
	Write()
	Remove()
	equal(row) bool
}

type table struct {
	rows map[int]row

	triggers map[Trigger]struct{}
	trigSeq  int
	seq      int
}

type transaction struct {
	do   func(db *Database) error
	done chan error
}

// A Conn is a database handle on which transactions may be executed.
type Conn chan transaction

// New creates a connection to a brand new database.
func New() Conn {
	cn := make(Conn)
	go cn.run()
	return cn
}

// Transact executes database transactions.  It takes a closure, 'do', which is operates
// on its 'db' argument.  Transactions are not concurrent, instead each runs sequentially
// on it's database without conflicting with other transactions.
func (cn Conn) Transact(do func(db *Database) error) error {
	txn := transaction{do, make(chan error)}
	cn <- txn
	return <-txn.done
}

// Trigger registers a new database trigger that watches changes to 'tableName'.  Any
// change to the table, including row insertions, deletions, and modifications, will
// cause a notification on 'Trigger.C'.
func (cn Conn) Trigger(tableName string) Trigger {
	trigger := Trigger{C: make(chan struct{}, 1), stop: make(chan struct{})}
	cn.Transact(func(db *Database) error {
		var table *table
		switch tableName {
		case "Machine":
			table = &db.machine
		case "Cluster":
			table = &db.cluster
		default:
			/* This would be a serious bug in the caller. */
			panic("Undefined table")
		}
		table.triggers[trigger] = struct{}{}
		return nil
	})

	return trigger
}

// TriggerTick creates a trigger, similar to Trigger(), that additionally ticks once
// every N 'seconds'.  So that clients properly initialize, TriggerTick() sends an
// initialization tick at startup.
func (cn Conn) TriggerTick(tableName string, seconds int) Trigger {
	trigger := cn.Trigger(tableName)

	go func() {
		ticker := time.NewTicker(time.Duration(seconds) * time.Second)
		defer ticker.Stop()

		for {
			select {
			case trigger.C <- struct{}{}:
			default:
			}

			select {
			case <-ticker.C:
			case <-trigger.stop:
				return
			}
		}
	}()

	return trigger
}

// Stop a running trigger thus allowing resources to be deallocated.
func (t Trigger) Stop() {
	close(t.stop)
}

func (cn Conn) run() {
	db := Database{
		machine: newTable(),
		cluster: newTable(),
	}

	for txn := range cn {
		txn.done <- txn.do(&db)
		db.machine.alert()
		db.cluster.alert()
	}
}

func newTable() table {
	return table{
		rows:     make(map[int]row),
		triggers: make(map[Trigger]struct{}),
	}
}

func (db *Database) nextID() int {
	db.idAlloc++
	return db.idAlloc
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
