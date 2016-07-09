package stitch

import (
	"fmt"
	"strings"
)

type queryType int

const (
	// Simulate the failure of an individual container.
	removeOneQuery = iota
	// Simulate the failure of the container's availability set.
	removeSetQuery
)

type query struct {
	form   queryType
	target string // Node to remove.
	str    string // Original query text.
}

var queryFormKeywords map[string]queryType
var queryFormImpls map[queryType]func(
	graph graph,
	invs []invariant,
	query query,
) *invariant

func init() {
	queryFormKeywords = map[string]queryType{
		"removeOne": removeOneQuery,
		"removeSet": removeOneQuery,
	}

	queryFormImpls = map[queryType]func(
		graph graph,
		invs []invariant,
		query query,
	) *invariant{
		removeOneQuery: whatIfRemoveOne,
		removeSetQuery: whatIfRemoveSet,
	}
}

func ask(graph graph, invs []invariant, path string) (*query, *invariant, error) {
	queries, err := parseQueries(graph, path)
	if err != nil {
		return nil, nil, err
	}

	return askQueries(graph, invs, queries)
}

func askQueries(graph graph, invs []invariant, queries []query) (*query, *invariant,
	error) {
	for _, query := range queries {
		if val := queryFormImpls[query.form](graph, invs, query); val != nil {
			return &query,
				val,
				fmt.Errorf(
					"mutation %s failed invariant %s",
					query.str, val.str)
		}
	}

	return nil, nil, nil
}

func parseQueryLine(graph graph, line string) (query, error) {
	sp := strings.Split(line, " ")
	if _, ok := graph.nodes[sp[1]]; !ok {
		return query{}, fmt.Errorf("malformed query (unknown label): %s", sp[1])
	}

	if form, ok := queryFormKeywords[sp[0]]; ok {
		return query{form: form, target: sp[1], str: line}, nil
	}
	return query{}, fmt.Errorf("could not parse query: %s", line)
}

func parseQueries(graph graph, path string) ([]query, error) {
	var queries []query

	parse := func(line string) error {
		query, err := parseQueryLine(graph, line)
		if err != nil {
			return err
		}
		queries = append(queries, query)
		return nil
	}

	if err := forLineInFile(path, parse); err != nil {
		return queries, err
	}

	return queries, nil
}

func whatIfRemoveOne(graph graph, invs []invariant, query query) *invariant {
	graphCopy := graph.copyGraph()
	graphCopy.removeNode(query.target)

	if _, failer, err := checkInvariants(graphCopy, invs); err != nil {
		return failer
	}
	return nil
}

func whatIfRemoveSet(graph graph, invs []invariant, query query) *invariant {
	graphCopy := graph.copyGraph()
	node := query.target
	avSet := graphCopy.findAvailabilitySet(node)
	if avSet == nil {
		return &invariant{str: fmt.Sprintf(
			"could not find availability set: %s", node)}
	}

	graphCopy.removeAvailabiltySet(avSet)

	if _, failer, err := checkInvariants(graphCopy, invs); err != nil {
		return failer
	}
	return nil
}
