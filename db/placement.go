package db

import (
	"fmt"

	"github.com/NetSys/quilt/minion/docker"
)

type Placement struct {
	ID int

	TargetLabel string
	Rule        PlacementRule
}

// PlacementSlice is an alias for []Placement to allow for joins
type PlacementSlice []Placement

// Returns true if this placement applies to the container c, and false if it doesn't.
func (p Placement) Applies(c Container) bool {
	for _, label := range c.Labels {
		if label == p.TargetLabel {
			return true
		}
	}

	return false
}

type PlacementRule interface {
	// Return the affinity string that represents this rule
	AffinityStr() string

	fmt.Stringer
}

type LabelRule struct {
	OtherLabel string
	Exclusive  bool
}

func (lr LabelRule) AffinityStr() string {
	return toAffinity(docker.UserLabel(lr.OtherLabel), !lr.Exclusive, docker.LabelTrueValue)
}

func (lr LabelRule) String() string {
	return lr.AffinityStr()
}

type MachineRule struct {
	Attribute string
	Value     string
	Exclusive bool
}

func (mr MachineRule) AffinityStr() string {
	return toAffinity(docker.SystemLabel(mr.Attribute), !mr.Exclusive, mr.Value)
}

func (mr MachineRule) String() string {
	return mr.AffinityStr()
}

type PortRule struct {
	Port int
}

func (pr PortRule) AffinityStr() string {
	return toAffinity(docker.PortLabel(pr.Port), false, docker.LabelTrueValue)
}

func (pr PortRule) String() string {
	return pr.AffinityStr()
}

func toAffinity(left string, eq bool, right string) string {
	eqStr := "!="
	if eq {
		eqStr = "=="
	}
	return fmt.Sprintf("affinity:%s%s%s", left, eqStr, right)
}

// InsertPlacement creates a new placement row and inserts it into the database.
func (db Database) InsertPlacement() Placement {
	result := Placement{ID: db.nextID()}
	db.insert(result)
	return result
}

// SelectFromPlacement gets all placements in the database that satisfy 'check'.
func (db Database) SelectFromPlacement(check func(Placement) bool) []Placement {
	var result []Placement
	for _, row := range db.tables[PlacementTable].rows {
		if check == nil || check(row.(Placement)) {
			result = append(result, row.(Placement))
		}
	}

	return result
}

// SelectFromPlacement gets all placements in the database that satisfy the 'check'.
func (conn Conn) SelectFromPlacement(check func(Placement) bool) []Placement {
	var placements []Placement
	conn.Transact(func(view Database) error {
		placements = view.SelectFromPlacement(check)
		return nil
	})
	return placements
}

func (p Placement) String() string {
	return defaultString(p)
}

func (p Placement) less(r row) bool {
	return p.ID < r.(Placement).ID
}

func (p Placement) getID() int {
	return p.ID
}

func (p Placement) equal(r row) bool {
	return p == r.(Placement)
}

// Get returns the value contained at the given index
func (ps PlacementSlice) Get(ii int) interface{} {
	return ps[ii]
}

// Len returns the numebr of items in the slice
func (ps PlacementSlice) Len() int {
	return len(ps)
}
