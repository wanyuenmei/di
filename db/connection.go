package db

import "fmt"

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

func (c Connection) id() int {
	return c.ID
}

func (c Connection) tt() TableType {
	return ConnectionTable
}

func (c Connection) String() string {
	port := fmt.Sprintf("%d", c.MinPort)
	if c.MaxPort == c.MinPort {
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
