package main

import (
	"fmt"
	"strings"
)

type invariantType int

const (
	// Reachability (reach): two arguments, <from> <to...>
	reachInvariant = iota
	// On-pathness (between): three arguments, <from> <to> <between>
	betweenInvariant
	// Schedulability (enough): zero arguments
	schedulabilityInvariant
)

type invariant struct {
	form   invariantType
	target bool     // Desired answer to invariant question.
	nodes  []string // Nodes the invariant operates on.
	str    string   // Original invariant text.
}

var formKeywords map[string]invariantType
var formImpls map[invariantType]func(graph graph, inv invariant) bool

func init() {
	formKeywords = map[string]invariantType{
		"reach":   reachInvariant,
		"between": betweenInvariant,
		"enough":  schedulabilityInvariant,
	}

	formImpls = map[invariantType]func(graph graph, inv invariant) bool{
		reachInvariant:          reachImpl,
		betweenInvariant:        betweenImpl,
		schedulabilityInvariant: schedulabilityImpl,
	}
}

func check(graph graph, path string) ([]invariant, *invariant, error) {
	invs, err := parseInvariants(graph, path)
	if err != nil {
		return []invariant{}, nil, err
	}

	return checkInvariants(graph, invs)
}

func checkInvariants(graph graph, invs []invariant) ([]invariant, *invariant, error) {
	for _, asrt := range invs {
		if val := formImpls[asrt.form](graph, asrt); !val {
			return invs, &asrt, fmt.Errorf("invariant failed")
		}
	}

	return invs, nil, nil
}

func parseLine(graph graph, line string) (invariant, error) {
	sp := strings.Split(line, " ")
	var nodes []string

	// Validate target argument.
	target := true
	switch {
	case len(sp) == 0:
		return invariant{}, fmt.Errorf("split string returned zero length list")
	case len(sp) == 1:
		return invariant{
			form:   formKeywords[sp[0]],
			target: true,
			nodes:  nodes,
			str:    line,
		}, nil
	case len(sp) >= 2:
		switch sp[1] {
		case "true":
			target = true
		case "false":
			target = false
		default:
			return invariant{}, fmt.Errorf(
				"malformed assertion"+
					" (second argument must be one of "+
					"\"true\",\"false\"): %s",
				line,
			)
		}

		// Validate label arguments.
		for _, n := range sp[2:] {
			if _, ok := graph.nodes[n]; !ok {
				return invariant{}, fmt.Errorf(
					"malformed assertion (unknown label): %s",
					n,
				)
			}
			nodes = append(nodes, n)
		}

		return invariant{
			form:   formKeywords[sp[0]],
			target: target,
			nodes:  nodes,
			str:    line,
		}, nil
	}
	return invariant{}, fmt.Errorf("could not parse invariant")
}

// Invariant format: <form> <target value ("true"/"false")> <node labels...>
func parseInvariants(graph graph, path string) ([]invariant, error) {
	var invs []invariant

	parse := func(line string) error {
		inv, err := parseLine(graph, line)
		if err != nil {
			return err
		}
		invs = append(invs, inv)
		return nil
	}

	if err := forLineInFile(path, parse); err != nil {
		return invs, err
	}

	return invs, nil
}

func reachImpl(graph graph, inv invariant) bool {
	from, ok := graph.nodes[inv.nodes[0]]
	if !ok {
		return ok == inv.target
	}

	return contains(from.dfs(), inv.nodes[1]) == inv.target
}

func betweenImpl(graph graph, inv invariant) bool {
	from, ok := graph.nodes[inv.nodes[0]]
	if !ok {
		return ok == inv.target
	}
	to, ok := graph.nodes[inv.nodes[1]]
	if !ok {
		return ok == inv.target
	}

	paths, ok := paths(from, to)
	if !ok {
		// No path between source and dest.
		return !inv.target
	}

	betweenNode := inv.nodes[2]

	// True: betweenNode must be in all paths.
	if inv.target {
		for _, path := range paths {
			if !contains(path, betweenNode) {
				return false
			}
		}
		return true
	}
	// The betweenNode must not be in any path.
	for _, path := range paths {
		if contains(path, betweenNode) {
			return true
		}
	}
	return false
}

func schedulabilityImpl(graph graph, inv invariant) bool {
	machines := graph.machines
	avSets := graph.availability
	if _, ok := graph.nodes["public"]; ok {
		return len(machines) >= (len(avSets) - 1)
	}
	return len(machines) >= len(avSets)
}
