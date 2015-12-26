package main

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/NetSys/di/db"
	"github.com/NetSys/di/minion/pb"
	"github.com/davecgh/go-spew/spew"
)

func BenchmarkContainerDiff(b *testing.B) {
	conn := db.New()

	/* Ep0 and ep0 are arrays of endpoints. Each endpoint in ep0 has 10 labels.
	 * each element in ep0 corresponds to an element in ep0, with 1 label removed.
	 * Thus each endpoint will have a single label addition or removal on each
	 * transaction. */
	var ep0, ep1 []*pb.Endpoint
	for i := 0; i < 5000; i++ {
		var labels []string
		for j := 0; j < 10; j++ {
			labels = append(labels, fmt.Sprintf("%s", rand.Int63()))
		}
		ep0 = append(ep0, &pb.Endpoint{pb.Endpoint_Container, labels})
		ep1 = append(ep1, &pb.Endpoint{pb.Endpoint_Container, labels[1:]})
	}

	conn.Transact(func(view db.Database) error {
		setEndpointsTxn(view, ep1)
		return nil
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ep := ep0
		if i%2 == 1 {
			ep = ep1
		}
		conn.Transact(func(view db.Database) error {
			setEndpointsTxn(view, ep)
			return nil
		})
	}
}

func TestEndpointTxn(t *testing.T) {
	conn := db.New()
	trigg := conn.Trigger(db.ContainerTable).C

	if err := testEndpointTxn(conn); err != "" {
		t.Error(err)
	}
	if fired(trigg) {
		t.Error("Unexpected Database Change")
	}

	if err := testEndpointTxn(conn, []string{"a"}); err != "" {
		t.Error(err)
	}
	if !fired(trigg) {
		t.Error("Expected Database Change")
	}

	if err := testEndpointTxn(conn, []string{"a"}); err != "" {
		t.Error(err)
	}
	if fired(trigg) {
		t.Error("Unexpected Database Change")
	}

	if err := testEndpointTxn(conn, []string{"a"}, []string{"a", "b"}); err != "" {
		t.Error(err)
	}
	if !fired(trigg) {
		t.Error("Expected Database Change")
	}

	if err := testEndpointTxn(conn, []string{"a"}, []string{"a"}); err != "" {
		t.Error(err)
	}
	if !fired(trigg) {
		t.Error("Expected Database Change")
	}

	if err := testEndpointTxn(conn, []string{"a"}); err != "" {
		t.Error(err)
	}
	if !fired(trigg) {
		t.Error("Expected Database Change")
	}

	if err := testEndpointTxn(conn, []string{"a", "b"}, []string{"a", "c"}); err != "" {
		t.Error(err)
	}
	if !fired(trigg) {
		t.Error("Expected Database Change")
	}

	if err := testEndpointTxn(conn, []string{"a", "b"}, []string{"a", "c"}); err != "" {
		t.Error(err)
	}
	if fired(trigg) {
		t.Error("Unexpected Database Change")
	}
}

func testEndpointTxn(conn db.Conn, exp ...[]string) string {
	var eps []*pb.Endpoint
	for _, labels := range exp {
		eps = append(eps, &pb.Endpoint{pb.Endpoint_Container, labels})
	}

	var containers []db.Container
	conn.Transact(func(view db.Database) error {
		setEndpointsTxn(view, eps)
		containers = view.SelectFromContainer(nil)
		return nil
	})

	for _, e := range exp {
		found := false
		for i, c := range containers {
			if editDistance(c.Labels, e) == 0 {
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
