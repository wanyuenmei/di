package stitch

import (
	"bufio"
	"fmt"
	"os"
	"text/scanner"

	"github.com/NetSys/quilt/db"
)

const quiltPath = "QUILT_PATH"

type argOption struct {
	result interface{} // Optionally store result.
}

func usage() {
	fmt.Fprintln(os.Stderr, "Usage: quilt inspect <path to spec file> [commands]\n"+
		"Options\n"+
		" - viz <pdf|ascii>\n"+
		" - check <path to invariants file>\n"+
		" - query [must have check] <path to query file>\n"+
		"Dependencies\n"+
		" - easy-graph (install Graph::Easy from cpan)\n"+
		" - graphviz (install from your favorite package manager)\n")
	os.Exit(1)
}

// InspectMain is the main function for the inspect tool.
func InspectMain(args []string) {
	if len(args) < 3 {
		fmt.Println("not enough arguments: ", len(args)-1)
		usage()
	}

	configPath := args[1]

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
	pathStr, _ := os.LookupEnv(quiltPath)
	spec, err := New(*sc.Init(bufio.NewReader(f)), pathStr, false)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	containers := map[int]*db.Container{}
	for _, c := range spec.QueryContainers() {
		containers[c.ID] = &db.Container{
			Command: c.Command,
			Image:   c.Image,
			Env:     c.Env,
		}
	}

	containerLabels := make(map[string][]*db.Container)
	for label, ids := range spec.QueryLabels() {
		var slice []*db.Container
		for _, id := range ids {
			slice = append(slice, containers[id])
		}
		containerLabels[label] = slice
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

	ignoreNext := 0
	foundFlags := map[string]argOption{}
	func() {
		args := args[2:]
		for i, arg := range args {
			switch {
			case ignoreNext > 0:
				ignoreNext--
			case arg == "viz":
				foundFlags[arg] = argOption{}
				outputFormat := args[i+1] // 'pdf' or 'ascii'
				viz(configPath, spec, graph, outputFormat)
				ignoreNext = 1
			case arg == "check":
				invs, failer, err := check(graph, args[i+1])
				if err != nil && failer == nil {
					fmt.Printf("parsing invariants failed: %s", err)
				} else if err != nil {
					fmt.Println("invariant failed: ", failer.str)
				} else {
					fmt.Println("invariants passed")
				}
				foundFlags[arg] = argOption{result: invs}
				ignoreNext = 1
			case arg == "query":
				foundFlags[arg] = argOption{}

				defer func(i int) {
					if checkOpt, ok := foundFlags["check"]; !ok {
						fmt.Println("query without check")
						usage()
					} else {
						invs := checkOpt.result.([]invariant)
						_, _, err := ask(graph, invs, args[i+1])
						if err != nil {
							fmt.Println(err)
						} else {
							fmt.Println("query passed " +
								"invariants")
						}
					}
				}(i)
				ignoreNext = 1
			default:
				fmt.Println("unknown arg", arg)
				usage()
			}
		}
	}()
}
