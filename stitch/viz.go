package stitch

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/NetSys/quilt/db"
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

	cmap := map[int]*db.Container{}
	for _, container := range spec.QueryContainers() {
		cmap[container.ID] = &db.Container{
			Image:   container.Image,
			Command: container.Command,
			Env:     container.Env,
		}
	}

	containerLabels := map[string][]*db.Container{}
	for label, ids := range spec.QueryLabels() {
		var containers []*db.Container
		for _, id := range ids {
			containers = append(containers, cmap[id])
		}
		containerLabels[label] = containers
	}

	graphviz(outputFormat, slug, graph, containerLabels)
}

func getImageNamesForLabel(containerLabels map[string][]*db.Container,
	label string) (imageNames string) {
	containers := containerLabels[label]
	if len(containers) == 1 {
		return fmt.Sprintf("\"%s: %s\"", label, containers[0].Image)
	}

	containerGroup := make(map[string]int)
	for _, c := range containers {
		count, here := containerGroup[c.Image]
		if !here {
			containerGroup[c.Image] = 1
		} else {
			containerGroup[c.Image] = count + 1
		}
	}

	images := ""
	for imageName, count := range containerGroup {
		images += fmt.Sprintf("%d %s ", count, imageName)
	}
	return fmt.Sprintf("\" %s: [ %s]\"", label, images)
}

// Graphviz generates a specification for the graphviz program that visualizes the
// communication graph of a stitch.
func graphviz(outputFormat string, slug string, graph graph,
	containerLabels map[string][]*db.Container) {
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
		dotfile += subGraph(containerLabels, i, av.nodes()...)
	}

	for _, edge := range graph.getConnections() {
		dotfile +=
			fmt.Sprintf(
				"    %s -> %s\n",
				getImageNamesForLabel(containerLabels, edge.from),
				getImageNamesForLabel(containerLabels, edge.to),
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
	writeGraph.CombinedOutput()
}

func subGraph(
	containerLabels map[string][]*db.Container,
	i int,
	labels ...string,
) string {
	subgraph := fmt.Sprintf("    subgraph cluster_%d {\n", i)
	str := ""
	for _, l := range labels {
		str += getImageNamesForLabel(containerLabels, l) + "; "
	}
	subgraph += "        " + str + "\n    }\n"
	return subgraph
}
