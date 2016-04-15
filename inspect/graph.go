package main

type Node struct {
	Name        Label
	Connections map[string]*Node
}

type Connection struct {
	From *Node
	To   *Node
}

type Graph struct {
	Nodes       map[string]Node
	Connections []Connection
}

func makeGraph() Graph {
	return Graph{
		Nodes:       make(map[string]Node),
		Connections: make([]Connection, 0),
	}
}

func (g Graph) getNode(label string) *Node {
	node, have := g.Nodes[label]
	if !have {
		node = Node{
			Name:        Label(label),
			Connections: make(map[string]*Node),
		}
		g.Nodes[label] = node
	}

	return &node
}

func (g *Graph) addConnection(from string, to string) {
	fromNode := g.getNode(from)
	toNode := g.getNode(to)
	fromNode.Connections[to] = toNode
	g.Connections = append(g.Connections, Connection{From: fromNode, To: toNode})
}

// find all nodes reachable from the given node
func (n *Node) dfs() []Label {
	reached := make(map[string]struct{})

	var explore func(t *Node)
	explore = func(t *Node) {
		for label, node := range t.Connections {
			if Label(label) != n.Name {
				_, explored := reached[label]
				if !explored {
					reached[label] = struct{}{}
					explore(node)
				}
			}
		}
	}
	explore(n)

	reachable := make([]Label, 0)
	for l := range reached {
		reachable = append(reachable, Label(l))
	}

	return reachable
}

// compute all the paths between two nodes
func paths(start *Node, end *Node) ([]Path, bool) {
	reach := start.dfs()
	if !contains(reach, end.Name) {
		return nil, false
	}

	paths := make([]Path, 0)

	var explore func(t *Node, p Path)
	explore = func(t *Node, p Path) {
		if t.Name == end.Name {
			paths = append(paths, p)
			return
		}

		for label, node := range t.Connections {
			if !contains(p, Label(label)) { // no loops
				explore(node, append(p, Label(label)))
			}
		}
	}
	explore(start, Path{start.Name})
	return paths, true
}
