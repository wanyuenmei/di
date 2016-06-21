package db

import (
	"fmt"

	"github.com/NetSys/quilt/minion/docker"
)

// Placement represents a declaration about how containers should be placed.  These
// directives can be made either relative to labels of other containers, or Machines
// those containers run on.
type Placement struct {
	ID int

	TargetLabel string
	Rule        PlacementRule
}

// PlacementSlice is an alias for []Placement to allow for joins
type PlacementSlice []Placement

// Applies returns true if this placement applies to the container c, false otherwise.
func (p Placement) Applies(c Container) bool {
	for _, label := range c.Labels {
		if label == p.TargetLabel {
			return true
		}
	}

	return false
}

// A PlacementRule represents a declaration constraining container placement.
type PlacementRule interface {
	// Return the affinity string that represents this rule
	AffinityStr() string

	fmt.Stringer
}

// A LabelRule constrains placement relative to other container labels.
type LabelRule struct {
	OtherLabel string
	Exclusive  bool
}

// AffinityStr is passed to Docker Swarm to implement the LabelRule.
func (lr LabelRule) AffinityStr() string {
	return toAffinity(docker.UserLabel(lr.OtherLabel), !lr.Exclusive,
		docker.LabelTrueValue)
}

// String returns the AffinityStr of this label.
func (lr LabelRule) String() string {
	return lr.AffinityStr()
}

// A MachineRule constrains container placement relative to the machine it runs on.
type MachineRule struct {
	Attribute string
	Value     string
	Exclusive bool
}

// AffinityStr is passed to Docker Swarm to implement the MachineRule.
func (mr MachineRule) AffinityStr() string {
	return toAffinity(docker.SystemLabel(mr.Attribute), !mr.Exclusive, mr.Value)
}

// String returns the AffinityStr of 'mr'.
func (mr MachineRule) String() string {
	return mr.AffinityStr()
}

// A PortRule constrains container placement relative to public network ports they will
// need.
type PortRule struct {
	Port int
}

// AffinityStr is passed to Docker Swarm to implement the PortRule.
func (pr PortRule) AffinityStr() string {
	return toAffinity(docker.PortLabel(pr.Port), false, docker.LabelTrueValue)
}

// String returns the AffinityStr of 'pr'.
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
