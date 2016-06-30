package stitch

import (
	"fmt"
)

// A Node in the communiction Graph.
type Node struct {
	Name        string
	Label       string
	Connections map[string]Node
}

// An Edge in the communication Graph.
type Edge struct {
	From string
	To   string
}

// A Graph represents permission to communicate across a series of Nodes.
// Each Node is a container and each edge is permissions to
// initiate a connection.
type Graph struct {
	Nodes map[string]Node
	// A set of containers which can be placed together on a VM.
	Availability []AvailabilitySet
	// Constraints on which containers can be placed together.
	Placement map[string][]string
	Machines  []Machine
}

// InitializeGraph queries the Stitch to fill in the Graph structure.
func InitializeGraph(spec Stitch) (Graph, error) {
	g := Graph{
		Nodes: map[string]Node{},
		// One global availability set by default.
		Availability: []AvailabilitySet{{}},
		Placement:    map[string][]string{},
		Machines:     []Machine{},
	}

	for label, cids := range spec.QueryLabels() {
		for _, cid := range cids {
			g.addNode(fmt.Sprintf("%d", cid), label)
		}
	}
	g.addNode(PublicInternetLabel, PublicInternetLabel)

	for _, conn := range spec.QueryConnections() {
		err := g.addConnection(conn.From, conn.To)
		if err != nil {
			return Graph{}, err
		}
	}

	for _, pl := range spec.QueryPlacements() {
		err := g.addPlacementRule(pl)
		if err != nil {
			return Graph{}, err
		}
	}

	for _, m := range spec.QueryMachines() {
		g.Machines = append(g.Machines, m)
	}

	return g, nil
}

// GetConnections returns a list of the edges in the Graph.
func (g Graph) GetConnections() []Edge {
	var res []Edge
	for _, n := range g.getNodes() {
		for _, edge := range n.Connections {
			res = append(res, Edge{From: n.Name, To: edge.Name})
		}
	}
	return res
}

func (g Graph) copyGraph() Graph {
	newNodes := map[string]Node{}
	for label, node := range g.Nodes {
		newNodes[label] = node
	}

	newAvail := make([]AvailabilitySet, len(g.Availability))
	copy(newAvail, g.Availability)

	return Graph{Nodes: newNodes, Availability: newAvail}
}

func (g *Graph) addConnection(from string, to string) error {
	// from and to are labels
	var fromContainers []Node
	var toContainers []Node

	for _, node := range g.Nodes {
		if node.Label == from {
			fromContainers = append(fromContainers, node)
		}
		if node.Label == to {
			toContainers = append(toContainers, node)
		}
	}

	for _, fromNode := range fromContainers {
		for _, toNode := range toContainers {
			if fromNode.Name != toNode.Name {
				fromNode.Connections[toNode.Name] = toNode
			}
		}
	}

	return nil
}

func (g Graph) getNodes() []Node {
	var res []Node
	for _, n := range g.Nodes {
		res = append(res, n)
	}
	return res
}

func (g *Graph) addNode(cid string, label string) Node {
	n := Node{
		Name:        cid,
		Label:       label,
		Connections: map[string]Node{},
	}
	g.Nodes[cid] = n
	g.Availability[0].Insert(cid)
	g.placeNodes()

	return n
}

func (g *Graph) removeNode(label string) {
	delete(g.Nodes, label)

	// Delete edges to this Node.
	for _, n := range g.getNodes() {
		delete(n.Connections, label)
	}
}

// Find all nodes reachable from the given node.
func (n Node) dfs() []string {
	reached := map[string]struct{}{}

	var explore func(t Node)
	explore = func(t Node) {
		for label, node := range t.Connections {
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

// Compute all the paths between two Nodes.
func paths(start Node, end Node) ([][]string, bool) {
	reach := start.dfs()
	if !contains(reach, end.Name) {
		return nil, false
	}

	var paths [][]string

	var explore func(t Node, p []string)
	explore = func(t Node, p []string) {
		if t.Name == end.Name {
			paths = append(paths, p)
			return
		}

		for label, node := range t.Connections {
			if !contains(p, label) { // Discount self-reachability.
				explore(node, append(p, label))
			}
		}
	}
	explore(start, []string{start.Name})
	return paths, true
}
