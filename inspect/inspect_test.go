package main

import (
	"bufio"
	"os"
	"testing"
	"text/scanner"

	"github.com/NetSys/quilt/stitch"
)

func initSpec(configPath string) graph {
	f, err := os.Open(configPath)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	sc := scanner.Scanner{
		Position: scanner.Position{
			Filename: configPath,
		},
	}
	spec, err := stitch.New(*sc.Init(bufio.NewReader(f)), "../specs")
	if err != nil {
		panic(err)
	}

	graph := makeGraph()
	for _, conn := range spec.QueryConnections() {
		graph.addConnection(conn.From, conn.To)
	}

	for _, pl := range spec.QueryPlacements() {
		graph.addPlacementRule(pl)
	}

	for _, m := range spec.QueryMachines() {
		graph.machines = append(graph.machines, m)
	}

	return graph
}

func runInvTest(sp string, invp string) (string, error) {
	graph := initSpec(sp)
	_, failer, err := check(graph, invp)
	if err != nil && failer == nil {
		return "", err
	} else if err != nil {
		return failer.str, err
	} else {
		return "", nil
	}
}

func TestReach(t *testing.T) {
	specPath := "./test/reach.spec"
	invariantPath := "./test/reach.inv"
	// Correct result: all invariants pass.
	if failer, err := runInvTest(specPath, invariantPath); err != nil ||
		failer != "" {
		t.Errorf("%s %s", err, failer)
	}
}

func TestBetween(t *testing.T) {
	specPath := "./test/between.spec"
	invariantPath := "./test/between.inv"
	// Correct result: all invariants pass.
	if failer, err := runInvTest(specPath, invariantPath); err != nil ||
		failer != "" {
		t.Errorf("%s %s", err, failer)
	}
}

func TestPlacement(t *testing.T) {
	specPath := "./test/placement.spec"
	invariantPath := "./test/placement.inv"
	// Correct result: invariant "between true a e d" fails.
	expectedFailure := "between true a e d"
	if failer, err := runInvTest(specPath, invariantPath); err == nil ||
		failer != expectedFailure {
		t.Errorf("%s %s", err, failer)
	}
}

func TestEnough(t *testing.T) {
	t.Skip("Skipping due to transient failures.  Will revisit this soon.")
	specPath := "./test/placement.spec"
	invariantPath := "./test/enough.inv"
	// correct result: all invariants pass
	if failer, err := runInvTest(specPath, invariantPath); err != nil ||
		failer != "" {
		t.Errorf("%s %s", err, failer)
	}
}

func TestQueryPlacement(t *testing.T) {
	specPath := "./test/placement.spec"
	invariantPath := "./test/placement.inv"
	queryPath := "./test/placement.qry"

	graph := initSpec(specPath)
	invs, _, _ := check(graph, invariantPath)
	// Correct result: all invariants pass.
	if _, _, err := ask(graph, invs, queryPath); err != nil {
		t.Errorf("%s", err)
	}
}
