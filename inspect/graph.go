package main

// A node in the communiction graph.
type node struct {
	name        string
	connections map[string]node
}

// A connection is an edge in the communication graph.
type connection struct {
	from node
	to   node
}

// A graph represents permission to communicate across a series of nodes.
// Each node is a container and each edge is permissions to
// initiate a connection.
type graph struct {
	nodes       map[string]node
	connections []connection
}

func makeGraph() graph {
	return graph{
		nodes:       map[string]node{},
		connections: []connection{},
	}
}

func (g *graph) getAddNode(label string) node {
	foundNode, ok := g.nodes[label]
	if !ok {
		n := node{
			name:        label,
			connections: map[string]node{},
		}
		g.nodes[label] = n
		foundNode = n
	}

	return foundNode
}

func (g *graph) addConnection(from string, to string) {
	fromNode := g.getAddNode(from)
	toNode := g.getAddNode(to)
	fromNode.connections[to] = toNode
	g.connections = append(g.connections, connection{from: fromNode, to: toNode})
}

// find all nodes reachable from the given node
func (n node) dfs() []string {
	reached := map[string]struct{}{}

	var explore func(t node)
	explore = func(t node) {
		for label, node := range t.connections {
			_, explored := reached[label]
			if !explored {
				reached[label] = struct{}{}
				explore(node)
			}
		}
	}
	explore(n)

	var reachable []string
	for l := range reached {
		reachable = append(reachable, string(l))
	}

	return reachable
}

// compute all the paths between two nodes
func paths(start node, end node) ([][]string, bool) {
	reach := start.dfs()
	if !contains(reach, end.name) {
		return nil, false
	}

	var paths [][]string

	var explore func(t node, p []string)
	explore = func(t node, p []string) {
		if t.name == end.name {
			paths = append(paths, p)
			return
		}

		for label, node := range t.connections {
			if !contains(p, label) { // no loops
				explore(node, append(p, label))
			}
		}
	}
	explore(start, []string{start.name})
	return paths, true
}
