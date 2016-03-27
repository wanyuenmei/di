package db

// A Label row is created for each container specified by the policy.  Each row will
// eventually be instantiated within its corresponding cluster. */
type Label struct {
	ID int

	Label     string
	IP        string
	MultiHost bool
}

// InsertLabel creates a new container row and inserts it into the database.
func (db Database) InsertLabel() Label {
	result := Label{ID: db.nextID()}
	db.insert(result)
	return result
}

// SelectFromLabel gets all containers in the database that satisfy 'check'.
func (db Database) SelectFromLabel(check func(Label) bool) []Label {
	var result []Label
	for _, row := range db.tables[LabelTable].rows {
		if check == nil || check(row.(Label)) {
			result = append(result, row.(Label))
		}
	}

	return result
}

// SelectFromLabel gets all containers in the database connection that satisfy 'check'.
func (conn Conn) SelectFromLabel(check func(Label) bool) []Label {
	var result []Label
	conn.Transact(func(view Database) error {
		result = view.SelectFromLabel(check)
		return nil
	})
	return result
}

func (r Label) String() string {
	return defaultString(r)
}

func (r Label) less(row row) bool {
	r2 := row.(Label)

	switch {
	case r.Label != r2.Label:
		return r.Label < r2.Label
	default:
		return r.ID < r2.ID
	}
}
