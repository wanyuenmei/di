package stitch

import (
	"fmt"
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

func (i invariant) String() string {
	return i.str
}

func (i invariant) eval(ctx *evalCtx) (ast, error) {
	return i, nil
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

func checkInvariants(graph graph, invs []invariant) ([]invariant, *invariant, error) {
	for _, asrt := range invs {
		if val := formImpls[asrt.form](graph, asrt); !val {
			return invs, &asrt, fmt.Errorf("invariant failed")
		}
	}

	return invs, nil, nil
}

func reachImpl(graph graph, inv invariant) bool {
	var fromNodes []node
	var toNodes []node
	for _, node := range graph.nodes {
		if node.label == inv.nodes[0] {
			fromNodes = append(fromNodes, node)
		}
		if node.label == inv.nodes[1] {
			toNodes = append(toNodes, node)
		}
	}

	allPassed := true
	for _, from := range fromNodes {
		for _, to := range toNodes {
			pass := contains(from.dfs(), to.name) == inv.target
			allPassed = allPassed && pass
		}
	}

	return allPassed
}

func betweenImpl(graph graph, inv invariant) bool {
	var fromNodes []node
	var toNodes []node
	var betweenNodes []node
	for _, node := range graph.nodes {
		switch node.label {
		case inv.nodes[0]:
			fromNodes = append(fromNodes, node)
		case inv.nodes[1]:
			toNodes = append(toNodes, node)
		case inv.nodes[2]:
			betweenNodes = append(betweenNodes, node)
		}
	}

	allPassed := true
	for _, from := range fromNodes {
		for _, to := range toNodes {
			allPassed = allPassed && betweenPathsHelper(
				betweenNodes,
				from,
				to,
				inv.target,
			)
		}
	}
	return allPassed
}

func betweenPathsHelper(betweenNodes []node, from node, to node, target bool) bool {
	paths, ok := paths(from, to)
	if !ok {
		// No path between source and dest.
		return !target
	}

	if target { // A betweenNode must be in all paths.
		allPaths := true
	pathsAll:
		for _, path := range paths {
			for _, between := range betweenNodes {
				if ok := contains(path, between.name); ok {
					break
				} else {
					allPaths = false
					break pathsAll
				}
			}
		}
		return allPaths
	}
	// A betweenNode must not be in any path.
	noPaths := true
pathsAny:
	for _, path := range paths {
		for _, between := range betweenNodes {
			if ok := contains(path, between.name); ok {
				noPaths = false
				break pathsAny
			}
		}
	}
	return noPaths
}

func schedulabilityImpl(graph graph, inv invariant) bool {
	machines := graph.machines
	avSets := graph.availability
	if _, ok := graph.nodes["public"]; ok {
		return len(machines) >= (len(avSets) - 1)
	}
	return len(machines) >= len(avSets)
}
