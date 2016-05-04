package main

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"
	"text/scanner"
	"time"

	"github.com/NetSys/di/db"
	"github.com/NetSys/di/dsl"
	"github.com/NetSys/di/util"
	"github.com/davecgh/go-spew/spew"
)

const testImage = "alpine"

func TestContainerTxn(t *testing.T) {
	conn := db.New()
	trigg := conn.Trigger(db.ContainerTable).C

	spec := ""
	if err := testContainerTxn(conn, spec); err != "" {
		t.Error(err)
	}
	if fired(trigg) {
		t.Error("Unexpected Database Change")
	}

	spec = `(label "a" (docker "alpine" "tail"))`
	if err := testContainerTxn(conn, spec); err != "" {
		t.Error(err)
	}
	if !fired(trigg) {
		t.Error("Expected Database Change")
	}

	if err := testContainerTxn(conn, spec); err != "" {
		t.Error(err)
	}
	if fired(trigg) {
		t.Error("Unexpected Database Change")
	}

	spec = `(label "b" (docker "alpine" "tail"))
		 (label "a" "b" (docker "alpine" "tail"))`
	if err := testContainerTxn(conn, spec); err != "" {
		t.Error(err)
	}
	if !fired(trigg) {
		t.Error("Expected Database Change")
	}

	spec = `(label "b" (docker "alpine" "cat"))
		 (label "a" "b" (docker "ubuntu" "tail"))`
	if err := testContainerTxn(conn, spec); err != "" {
		t.Error(err)
	}
	if !fired(trigg) {
		t.Error("Expected Database Change")
	}

	spec = `(label "b" (docker "ubuntu" "cat"))
		 (label "a" "b" (docker "alpine" "tail"))`
	if err := testContainerTxn(conn, spec); err != "" {
		t.Error(err)
	}
	if !fired(trigg) {
		t.Error("Expected Database Change")
	}

	spec = `(label "a" (makeList 2 (docker "alpine" "cat")))`
	if err := testContainerTxn(conn, spec); err != "" {
		t.Error(err)
	}
	if !fired(trigg) {
		t.Error("Expected Database Change")
	}

	spec = `(label "a" (docker "alpine"))`
	if err := testContainerTxn(conn, spec); err != "" {
		t.Error(err)
	}
	if !fired(trigg) {
		t.Error("Expected Database Change")
	}

	spec = `(label "b" (docker "alpine"))
	        (label "c" (docker "alpine"))
	        (label "a" "b" "c")`
	if err := testContainerTxn(conn, spec); err != "" {
		t.Error(err)
	}
	if !fired(trigg) {
		t.Error("Expected Database Change")
	}

	if err := testContainerTxn(conn, spec); err != "" {
		t.Error(err)
	}
	if fired(trigg) {
		t.Error("Unexpected Database Change")
	}
}

func testContainerTxn(conn db.Conn, spec string) string {
	var containers []db.Container
	conn.Transact(func(view db.Database) error {
		updatePolicy(view, db.Master, spec)
		containers = view.SelectFromContainer(nil)
		return nil
	})

	var sc scanner.Scanner
	compiled, err := dsl.New(*sc.Init(strings.NewReader(spec)), []string{})
	if err != nil {
		return err.Error()
	}

	exp := compiled.QueryContainers()
	for _, e := range exp {
		found := false
		for i, c := range containers {
			if e.Image == c.Image &&
				reflect.DeepEqual(e.Command, c.Command) &&
				util.EditDistance(c.Labels, e.Labels()) == 0 {
				containers = append(containers[:i], containers[i+1:]...)
				found = true
				break
			}
		}

		if found == false {
			return fmt.Sprintf("Missing expected label set: %s\n%s",
				e, containers)
		}
	}

	if len(containers) > 0 {
		return spew.Sprintf("Unexpected containers: %s", containers)
	}

	return ""
}

func TestConnectionTxn(t *testing.T) {
	conn := db.New()
	trigg := conn.Trigger(db.ConnectionTable).C

	spec := ""
	if err := testConnectionTxn(conn, spec); err != "" {
		t.Error(err)
	}
	if fired(trigg) {
		t.Error("Unexpected Database Change")
	}

	spec = `(label "a" (docker "alpine"))
	        (connect 80 "a" "a")`
	if err := testConnectionTxn(conn, spec); err != "" {
		t.Error(err)
	}
	if !fired(trigg) {
		t.Error("Expected Database Change")
	}
	if err := testConnectionTxn(conn, spec); err != "" {
		t.Error(err)
	}
	if fired(trigg) {
		t.Error("Unexpected Database Change")
	}

	spec = `(label "a" (docker "alpine"))
	        (connect 90 "a" "a")`
	if err := testConnectionTxn(conn, spec); err != "" {
		t.Error(err)
	}
	if !fired(trigg) {
		t.Error("Expected Database Change")
	}
	if err := testConnectionTxn(conn, spec); err != "" {
		t.Error(err)
	}
	if fired(trigg) {
		t.Error("Unexpected Database Change")
	}

	spec = `(label "a" (docker "alpine"))
                (label "b" (docker "alpine"))
                (label "c" (docker "alpine"))
	        (connect 90 "b" "a" "c")
	        (connect 100 "b" "b")
	        (connect 101 "c" "a")`
	if err := testConnectionTxn(conn, spec); err != "" {
		t.Error(err)
	}
	if !fired(trigg) {
		t.Error("Expected Database Change")
	}
	if err := testConnectionTxn(conn, spec); err != "" {
		t.Error(err)
	}
	if fired(trigg) {
		t.Error("Unexpected Database Change")
	}

	spec = `(label "a" (docker "alpine"))
                (label "b" (docker "alpine"))
                (label "c" (docker "alpine"))`
	if err := testConnectionTxn(conn, spec); err != "" {
		t.Error(err)
	}
	if !fired(trigg) {
		t.Error("Expected Database Change")
	}
	if err := testConnectionTxn(conn, spec); err != "" {
		t.Error(err)
	}
	if fired(trigg) {
		t.Error("Unexpected Database Change")
	}
}

func testConnectionTxn(conn db.Conn, spec string) string {
	var connections []db.Connection
	conn.Transact(func(view db.Database) error {
		updatePolicy(view, db.Master, spec)
		connections = view.SelectFromConnection(nil)
		return nil
	})

	var sc scanner.Scanner
	compiled, err := dsl.New(*sc.Init(strings.NewReader(spec)), []string{})
	if err != nil {
		return err.Error()
	}

	exp := compiled.QueryConnections()
	for e := range exp {
		found := false
		for i, c := range connections {
			if e.From == c.From && e.To == c.To && e.MinPort == c.MinPort &&
				e.MaxPort == c.MaxPort {
				connections = append(connections[:i], connections[i+1:]...)
				found = true
				break
			}
		}

		if found == false {
			return fmt.Sprintf("Missing expected connection: %v", e)
		}
	}

	if len(connections) > 0 {
		return spew.Sprintf("Unexpected connections: %s", connections)
	}

	return ""
}

func fired(c chan struct{}) bool {
	time.Sleep(5 * time.Millisecond)
	select {
	case <-c:
		return true
	default:
		return false
	}
}

type placementList []db.Placement

func (l placementList) Len() int {
	return len(l)
}

func (l placementList) Swap(i, j int) {
	l[i], l[j] = l[j], l[i]
}

func (l placementList) Less(i, j int) bool {
	left := l[i]
	right := l[j]
	switch {
	case left.ID != right.ID:
		return left.ID < right.ID
	case left.TargetLabel != right.TargetLabel:
		return left.TargetLabel < right.TargetLabel
	default:
		return left.Rule.AffinityStr() < right.Rule.AffinityStr()
	}
}

func TestPlacementTxn(t *testing.T) {
	conn := db.New()
	checkPlacement := func(spec string, exp ...db.Placement) {
		var placements []db.Placement
		conn.Transact(func(view db.Database) error {
			updatePolicy(view, db.Master, spec)
			res := view.SelectFromPlacement(nil)

			// Set the ID to 0 so that we can use reflect.DeepEqual.
			for _, p := range res {
				p.ID = 0
				placements = append(placements, p)
			}

			return nil
		})

		sort.Sort(placementList(placements))
		sort.Sort(placementList(exp))
		if !reflect.DeepEqual(placements, exp) {
			t.Errorf("Placement error in %s. Expected %v, got %v",
				spec, exp, placements)
		}
	}

	// Create an exclusive placement.
	spec := `(label "foo" (docker "foo"))
	(label "bar" (docker "bar"))
	(place (labelRule "exclusive" "foo") "bar")`
	checkPlacement(spec,
		db.Placement{
			TargetLabel: "bar",
			Rule: db.LabelRule{
				Exclusive:  true,
				OtherLabel: "foo",
			},
		},
	)

	// Change the placement from "exclusive" to "on".
	spec = `(label "foo" (docker "foo"))
	(label "bar" (docker "bar"))
	(place (labelRule "on" "foo") "bar")`
	checkPlacement(spec,
		db.Placement{
			TargetLabel: "bar",
			Rule: db.LabelRule{
				Exclusive:  false,
				OtherLabel: "foo",
			},
		},
	)

	// Add another placement constraint.
	spec = `(label "foo" (docker "foo"))
	(label "bar" (docker "bar"))
	(place (labelRule "on" "foo") "bar")
	(place (labelRule "exclusive" "bar") "bar")`
	checkPlacement(spec,
		db.Placement{
			TargetLabel: "bar",
			Rule: db.LabelRule{
				Exclusive:  false,
				OtherLabel: "foo",
			},
		},
		db.Placement{
			TargetLabel: "bar",
			Rule: db.LabelRule{
				Exclusive:  true,
				OtherLabel: "bar",
			},
		},
	)

	// Multiple placement targets.
	spec = `(label "foo" (docker "foo"))
	(label "bar" (docker "bar"))
	(label "qux" (docker "qux"))
	(place (labelRule "exclusive" "qux") "foo" "bar")`
	checkPlacement(spec,
		db.Placement{
			TargetLabel: "bar",
			Rule: db.LabelRule{
				Exclusive:  true,
				OtherLabel: "qux",
			},
		},
		db.Placement{
			TargetLabel: "foo",
			Rule: db.LabelRule{
				Exclusive:  true,
				OtherLabel: "qux",
			},
		},
	)

	// Multiple exclusive labels.
	spec = `(label "foo" (docker "foo"))
	(label "bar" (docker "bar"))
	(label "baz" (docker "baz"))
	(label "qux" (docker "qux"))
	(place (labelRule "exclusive" "foo" "bar") "baz" "qux")`
	checkPlacement(spec,
		db.Placement{
			TargetLabel: "baz",
			Rule: db.LabelRule{
				Exclusive:  true,
				OtherLabel: "foo",
			},
		},
		db.Placement{
			TargetLabel: "baz",
			Rule: db.LabelRule{
				Exclusive:  true,
				OtherLabel: "bar",
			},
		},
		db.Placement{
			TargetLabel: "qux",
			Rule: db.LabelRule{
				Exclusive:  true,
				OtherLabel: "foo",
			},
		},
		db.Placement{
			TargetLabel: "qux",
			Rule: db.LabelRule{
				Exclusive:  true,
				OtherLabel: "bar",
			},
		},
	)

	// Machine placement
	spec = `(label "foo" (docker "foo"))
	(place (machineRule "on" (size "m4.large")) "foo")`
	checkPlacement(spec,
		db.Placement{
			TargetLabel: "foo",
			Rule: db.MachineRule{
				Exclusive: false,
				Attribute: "size",
				Value:     "m4.large",
			},
		},
	)
}
