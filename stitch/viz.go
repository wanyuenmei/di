package stitch

import (
	"fmt"
	"os"
	"os/exec"
)

func viz(configPath string, spec Stitch, graph graph, outputFormat string) {
	var slug string
	for i, ch := range configPath {
		if ch == '.' {
			slug = configPath[:i]
			break
		}
	}
	if len(slug) == 0 {
		panic("Could not find proper output file name")
	}

	graphviz(outputFormat, slug, graph)
}

// Graphviz generates a specification for the graphviz program that visualizes the
// communication graph of a stitch.
func graphviz(outputFormat string, slug string, graph graph) {
	f, err := os.Create(slug + ".dot")
	if err != nil {
		panic(err)
	}
	defer f.Close()
	defer func() {
		rm := exec.Command("rm", slug+".dot")
		rm.Run()
	}()

	dotfile := "strict digraph {\n"

	for i, av := range graph.availability {
		dotfile += subGraph(i, av.nodes()...)
	}

	for _, edge := range graph.getConnections() {
		dotfile +=
			fmt.Sprintf(
				"    %s -> %s\n",
				edge.from,
				edge.to,
			)
	}

	dotfile += "}\n"

	f.Write([]byte(dotfile))

	// Dependencies:
	// - easy-graph (install Graph::Easy from cpan)
	// - graphviz (install from your favorite package manager)
	var writeGraph *exec.Cmd
	switch outputFormat {
	case "ascii":
		writeGraph = exec.Command("graph-easy", "--input="+slug+".dot",
			"--as_ascii")
	case "pdf":
		writeGraph = exec.Command("dot", "-Tpdf", "-o", slug+".pdf",
			slug+".dot")
	}
	writeGraph.Stdout = os.Stdout
	writeGraph.Run()
}

func subGraph(
	i int,
	labels ...string,
) string {
	subgraph := fmt.Sprintf("    subgraph cluster_%d {\n", i)
	str := ""
	for _, l := range labels {
		str += l + "; "
	}
	subgraph += "        " + str + "\n    }\n"
	return subgraph
}
