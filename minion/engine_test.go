package main

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"text/scanner"
	"time"

	"github.com/NetSys/di/db"
	"github.com/NetSys/di/dsl"
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
				editDistance(c.Labels, e.Labels()) == 0 {
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
	for _, e := range exp {
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

func TestEditDistance(t *testing.T) {
	if err := ed(nil, nil, 0); err != "" {
		t.Error(err)
	}

	if err := ed([]string{"a"}, nil, 1); err != "" {
		t.Error(err)
	}

	if err := ed(nil, []string{"a"}, 1); err != "" {
		t.Error(err)
	}

	if err := ed([]string{"a"}, []string{"a"}, 0); err != "" {
		t.Error(err)
	}

	if err := ed([]string{"b"}, []string{"a"}, 2); err != "" {
		t.Error(err)
	}

	if err := ed([]string{"b", "a"}, []string{"a"}, 1); err != "" {
		t.Error(err)
	}

	if err := ed([]string{"b", "a"}, []string{}, 2); err != "" {
		t.Error(err)
	}

	if err := ed([]string{"a", "b", "c"}, []string{"a", "b", "c"}, 0); err != "" {
		t.Error(err)
	}

	if err := ed([]string{"b", "c"}, []string{"a", "b", "c"}, 1); err != "" {
		t.Error(err)
	}

	if err := ed([]string{"b", "c"}, []string{"a", "c"}, 2); err != "" {
		t.Error(err)
	}
}

func ed(a, b []string, exp int) string {
	if ed := editDistance(a, b); ed != exp {
		return fmt.Sprintf("Distance(%s, %s) = %v, expected %v", a, b, ed, exp)
	}
	return ""
}
