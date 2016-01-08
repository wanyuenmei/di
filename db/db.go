package db

import (
	"reflect"
	"time"

	"github.com/op/go-logging"
)

// The Database is the central storage location for all state in the system.  The policy
// engine populates the database with a preferred state of the world, while various
// modules flesh out that policy with actual implementation details.
type Database struct {
	tables  map[TableType]*table
	idAlloc *int
}

// A Trigger sends notifications when anything in their corresponding table changes.
type Trigger struct {
	C    chan struct{} // The channel on which notifications are delivered.
	stop chan struct{}
}

type row interface {
	id() int
	tt() TableType
	less(row) bool
	String() string
}

type transaction struct {
	do   func(db Database) error
	done chan error
}

// A Conn is a database handle on which transactions may be executed.
type Conn chan transaction

var log = logging.MustGetLogger("database")

// New creates a connection to a brand new database.
func New() Conn {
	db := Database{make(map[TableType]*table), new(int)}
	for _, t := range allTables {
		db.tables[t] = newTable()
	}

	cn := make(Conn)
	go cn.run(db)
	cn.runLogger()
	return cn
}

func (cn Conn) run(db Database) {
	for txn := range cn {
		txn.done <- txn.do(db)
		for _, table := range db.tables {
			table.alert()
		}
	}
}

// Transact executes database transactions.  It takes a closure, 'do', which is operates
// on its 'db' argument.  Transactions are not concurrent, instead each runs sequentially
// on it's database without conflicting with other transactions.
func (cn Conn) Transact(do func(db Database) error) error {
	txn := transaction{do, make(chan error)}
	cn <- txn
	return <-txn.done
}

// Trigger registers a new database trigger that watches changes to 'tableName'.  Any
// change to the table, including row insertions, deletions, and modifications, will
// cause a notification on 'Trigger.C'.
func (cn Conn) Trigger(tt ...TableType) Trigger {
	trigger := Trigger{C: make(chan struct{}, 1), stop: make(chan struct{})}
	cn.Transact(func(db Database) error {
		for _, t := range tt {
			db.tables[t].triggers[trigger] = struct{}{}
		}
		return nil
	})

	return trigger
}

// TriggerTick creates a trigger, similar to Trigger(), that additionally ticks once
// every N 'seconds'.  So that clients properly initialize, TriggerTick() sends an
// initialization tick at startup.
func (cn Conn) TriggerTick(seconds int, tt ...TableType) Trigger {
	trigger := cn.Trigger(tt...)

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

func (db Database) insert(r row) {
	table := db.tables[r.tt()]
	table.seq++
	table.rows[r.id()] = r
}

func (db Database) Commit(r row) {
	rid := r.id()
	table := db.tables[r.tt()]
	old := table.rows[rid]

	if reflect.TypeOf(old) != reflect.TypeOf(r) {
		panic("Type Error")
	}

	if !reflect.DeepEqual(table.rows[rid], r) {
		table.rows[rid] = r
		table.seq++
	}
}

func (db Database) Remove(r row) {
	table := db.tables[r.tt()]
	delete(table.rows, r.id())
	table.seq++
}

func (db Database) nextID() int {
	*db.idAlloc += 1
	return *db.idAlloc
}

type rowSlice []row

func (rows rowSlice) Len() int {
	return len(rows)
}

func (rows rowSlice) Swap(i, j int) {
	rows[i], rows[j] = rows[j], rows[i]
}

func (rows rowSlice) Less(i, j int) bool {
	return rows[i].less(rows[j])
}
