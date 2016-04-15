package main

import (
	"bufio"
	"github.com/NetSys/di/dsl"
	"os"
	"strings"
	"text/scanner"
)

const diPathKey = "DI_PATH"

func main() {
	var configPath string
	switch len(os.Args) {
	case 2:
		configPath = os.Args[1]
	default:
		panic("Usage: dslinspect <path to spec file>")
	}

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
	pathStr, _ := os.LookupEnv(diPathKey)
	pathSlice := strings.Split(pathStr, ":")
	spec, err := dsl.New(*sc.Init(bufio.NewReader(f)), pathSlice)
	if err != nil {
		panic(err)
	}

	containerLabels := make(map[string][]*dsl.Container)
	for _, container := range spec.QueryContainers() {
		labels := container.Labels()
		for _, label := range labels {
			if _, have := containerLabels[label]; !have {
				containerLabels[label] = make([]*dsl.Container, 0)
			}
			containerLabels[label] = append(containerLabels[label], container)
		}
	}

	graph := makeGraph()
	for _, conn := range spec.QueryConnections() {
		graph.addConnection(conn.From, conn.To)
	}

	slug := ""
	for i, ch := range configPath {
		if !(ch == '.') {
			slug = configPath[:i+1]
		} else {
			break
		}
	}

	Graphviz(slug, graph, containerLabels)
}
