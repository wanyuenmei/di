package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/NetSys/quilt/stitch"
)

func viz(configPath string, spec stitch.Stitch, graph graph) {
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

	containerLabels := map[string][]*stitch.Container{}
	for _, container := range spec.QueryContainers() {
		labels := container.Labels()
		for _, label := range labels {
			if _, ok := containerLabels[label]; !ok {
				containerLabels[label] = make([]*stitch.Container, 0)
			}
			containerLabels[label] = append(containerLabels[label], container)
		}
	}

	graphviz(slug, graph, containerLabels)
}

// Write parsed Quilt graph to a graphviz dotfile.

func getImageNamesForLabel(containerLabels map[string][]*stitch.Container,
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
func graphviz(slug string, graph graph, containerLabels map[string][]*stitch.Container) {
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

	// Run "dot" (part of graphviz) on the dotfile to output the image.
	writepng := exec.Command("dot", "-Tpdf", "-o", slug+".pdf", slug+".dot")
	writepng.Run()
}

func subGraph(
	containerLabels map[string][]*stitch.Container,
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
