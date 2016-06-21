package db

import (
	"fmt"
)

// A Connection allows the members of two labels to speak to each other on the port
// range [MinPort, MaxPort] inclusive.
type Connection struct {
	ID int

	From    string
	To      string
	MinPort int
	MaxPort int
}

// InsertConnection creates a new connection row and inserts it into the database.
func (db Database) InsertConnection() Connection {
	result := Connection{ID: db.nextID()}
	db.insert(result)
	return result
}

// SelectFromConnection gets all connections in the database that satisfy 'check'.
func (db Database) SelectFromConnection(check func(Connection) bool) []Connection {
	var result []Connection
	for _, row := range db.tables[ConnectionTable].rows {
		if check == nil || check(row.(Connection)) {
			result = append(result, row.(Connection))
		}
	}

	return result
}

func (c Connection) equal(r row) bool {
	return c == r.(Connection)
}

func (c Connection) getID() int {
	return c.ID
}

// SelectFromConnection gets all connections in the database connection that satisfy
// the 'check'.
func (conn Conn) SelectFromConnection(check func(Connection) bool) []Connection {
	var connections []Connection
	conn.Transact(func(view Database) error {
		connections = view.SelectFromConnection(check)
		return nil
	})
	return connections
}

func (c Connection) String() string {
	port := fmt.Sprintf("%d", c.MinPort)
	if c.MaxPort != c.MinPort {
		port += fmt.Sprintf("-%d", c.MaxPort)
	}

	return fmt.Sprintf("Connection-%d{%s->%s:%s}", c.ID, c.From, c.To, port)
}

func (c Connection) less(r row) bool {
	o := r.(Connection)

	switch {
	case c.From != o.From:
		return c.From < o.From
	case c.To != o.To:
		return c.To < o.To
	case c.MaxPort != o.MaxPort:
		return c.MaxPort < o.MaxPort
	case c.MinPort != o.MaxPort:
		return c.MinPort < o.MinPort
	default:
		return c.ID < o.ID
	}
}

// ConnectionSlice is an alias for []Connection to allow for joins
type ConnectionSlice []Connection

// Get returns the value contained at the given index
func (cs ConnectionSlice) Get(ii int) interface{} {
	return cs[ii]
}

// Len returns the number of items in the slice.
func (cs ConnectionSlice) Len() int {
	return len(cs)
}
