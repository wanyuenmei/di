package inspect

import (
	"bufio"
	"fmt"
	"os"
	"text/scanner"

	"github.com/NetSys/quilt/stitch"
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

// Main is the main function for inspect tool. Helps visualize stitches.
func Main(opts []string) {
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
	spec, err := stitch.New(*sc.Init(bufio.NewReader(f)), pathStr, false)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	graph, err := stitch.InitializeGraph(spec)
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
