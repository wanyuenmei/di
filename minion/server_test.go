package main

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/NetSys/di/db"
	"github.com/NetSys/di/dsl"
	"github.com/davecgh/go-spew/spew"
)

const TEST_IMAGE = "alpine"

func TestEndpointTxn(t *testing.T) {
	conn := db.New()
	trigg := conn.Trigger(db.ContainerTable).C

	spec := ""
	if err := testEndpointTxn(conn, spec); err != "" {
		t.Error(err)
	}
	if fired(trigg) {
		t.Error("Unexpected Database Change")
	}

	spec = `(label "a" (atom docker "alpine"))`
	if err := testEndpointTxn(conn, spec); err != "" {
		t.Error(err)
	}
	if !fired(trigg) {
		t.Error("Expected Database Change")
	}

	if err := testEndpointTxn(conn, spec); err != "" {
		t.Error(err)
	}
	if fired(trigg) {
		t.Error("Unexpected Database Change")
	}

	spec = `(label "b" (atom docker "alpine"))
		 (label "a" "b" (atom docker "alpine"))`
	if err := testEndpointTxn(conn, spec); err != "" {
		t.Error(err)
	}
	if !fired(trigg) {
		t.Error("Expected Database Change")
	}

	spec = `(label "b" (atom docker "alpine"))
		 (label "a" "b" (atom docker "ubuntu"))`
	if err := testEndpointTxn(conn, spec); err != "" {
		t.Error(err)
	}
	if !fired(trigg) {
		t.Error("Expected Database Change")
	}

	spec = `(label "b" (atom docker "ubuntu"))
		 (label "a" "b" (atom docker "alpine"))`
	if err := testEndpointTxn(conn, spec); err != "" {
		t.Error(err)
	}
	if !fired(trigg) {
		t.Error("Expected Database Change")
	}

	spec = `(label "a" (makeList 2 (atom docker "alpine")))`
	if err := testEndpointTxn(conn, spec); err != "" {
		t.Error(err)
	}
	if !fired(trigg) {
		t.Error("Expected Database Change")
	}

	spec = `(label "a" (atom docker "alpine"))`
	if err := testEndpointTxn(conn, spec); err != "" {
		t.Error(err)
	}
	if !fired(trigg) {
		t.Error("Expected Database Change")
	}

	spec = `(label "b" (atom docker "alpine"))
	        (label "c" (atom docker "alpine"))
	        (label "a" "b" "c")`
	if err := testEndpointTxn(conn, spec); err != "" {
		t.Error(err)
	}
	if !fired(trigg) {
		t.Error("Expected Database Change")
	}

	if err := testEndpointTxn(conn, spec); err != "" {
		t.Error(err)
	}
	if fired(trigg) {
		t.Error("Unexpected Database Change")
	}
}

func testEndpointTxn(conn db.Conn, spec string) string {
	var containers []db.Container
	conn.Transact(func(view db.Database) error {
		UpdatePolicy(view, spec)
		containers = view.SelectFromContainer(nil)
		return nil
	})

	compiled, err := dsl.New(strings.NewReader(spec))
	if err != nil {
		return err.Error()
	}

	exp := compiled.QueryContainers()
	for _, e := range exp {
		found := false
		for i, c := range containers {
			if e.Image == c.Image && editDistance(c.Labels, e.Labels) == 0 {
				containers = append(containers[:i], containers[i+1:]...)
				found = true
				break
			}
		}

		if found == false {
			return fmt.Sprintf("Missing expected label set: %s", e)
		}
	}

	if len(containers) > 0 {
		return spew.Sprintf("Unexpected containers: %s", containers)
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
		return fmt.Sprintf("Distance(%s, %s) = %s, expected %s", a, b, ed, exp)
	}
	return ""
}
