package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/NetSys/quilt/stitch"
)

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
func Graphviz(slug string, graph Graph,
	containerLabels map[string][]*stitch.Container) {
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

	for _, edge := range graph.Connections {
		dotfile +=
			fmt.Sprintf(
				"    %s -> %s\n",
				getImageNamesForLabel(containerLabels, string(edge.From.Name)),
				getImageNamesForLabel(containerLabels, string(edge.To.Name)),
			)
	}

	dotfile += "}\n"

	f.Write([]byte(dotfile))

	// run "dot" (part of graphviz) on the dotfile to output the image
	writepng := exec.Command("dot", "-Tpdf", "-o", slug+".pdf", slug+".dot")
	writepng.Run()
}
