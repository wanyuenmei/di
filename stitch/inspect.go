package stitch

import (
	"bufio"
	"fmt"
	"os"
	"text/scanner"
)

const quiltPath = "QUILT_PATH"

func usage() {
	fmt.Fprintln(
		os.Stderr,
		`quilt inspect is a tool that helps visualize Stitch specifications.
Usage: quilt inspect <path to spec file> <pdf|ascii>
Dependencies
 - easy-graph (install Graph::Easy from cpan)
 - graphviz (install from your favorite package manager)`,
	)
	os.Exit(1)
}

func initializeGraph(spec Stitch) (graph, error) {
	g := makeGraph()

	for label, cids := range spec.QueryLabels() {
		for _, cid := range cids {
			g.addNode(fmt.Sprintf("%d", cid), label)
		}
	}
	g.addNode(PublicInternetLabel, PublicInternetLabel)

	for _, conn := range spec.QueryConnections() {
		err := g.addConnection(conn.From, conn.To)
		if err != nil {
			return graph{}, err
		}
	}

	for _, pl := range spec.QueryPlacements() {
		err := g.addPlacementRule(pl)
		if err != nil {
			return graph{}, err
		}
	}

	for _, m := range spec.QueryMachines() {
		g.machines = append(g.machines, m)
	}

	return g, nil
}

// InspectMain is the main function for inspect tool. Helps visualize stitches.
func InspectMain(opts []string) {
	if arglen := len(opts); arglen < 3 {
		fmt.Println("not enough arguments: ", arglen-1)
		usage()
	}

	configPath := opts[1]

	f, err := os.Open(configPath)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer f.Close()

	sc := scanner.Scanner{
		Position: scanner.Position{
			Filename: configPath,
		},
	}
	pathStr, _ := os.LookupEnv(quiltPath)
	spec, err := New(*sc.Init(bufio.NewReader(f)), pathStr, false)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	graph, err := initializeGraph(spec)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	switch opts[2] {
	case "pdf":
		fallthrough
	case "ascii":
		viz(configPath, spec, graph, opts[2])
	default:
		usage()
	}
}
