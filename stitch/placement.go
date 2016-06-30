package stitch

import (
	"fmt"
)

// AvailabilitySet represents a set of containers which can be placed together on a VM.
type AvailabilitySet map[string]struct{}

// Nodes returns the membership of the set.
func (avSet AvailabilitySet) Nodes() []string {
	var labels []string
	for l := range avSet {
		labels = append(labels, l)
	}
	return labels
}

// Str returns the string representation of the set.
func (avSet AvailabilitySet) Str() string {
	return fmt.Sprint(avSet.Nodes())
}

// Check checks set membership.
func (avSet AvailabilitySet) Check(label string) bool {
	_, ok := avSet[label]
	return ok
}

// CopyAvSet returns a copy of the set.
func (avSet AvailabilitySet) CopyAvSet() AvailabilitySet {
	newAvSet := map[string]struct{}{}
	for label := range avSet {
		newAvSet[label] = struct{}{}
	}
	return newAvSet
}

// Insert inserts labels into the set.
func (avSet AvailabilitySet) Insert(labels ...string) {
	for _, label := range labels {
		avSet[label] = struct{}{}
	}
}

// Remove removes labels from the set.
func (avSet AvailabilitySet) Remove(labels ...string) {
	for _, label := range labels {
		delete(avSet, label)
	}
}

func (avSet AvailabilitySet) removeAndCheck(labels ...string) []string {
	var toMove []string
	for _, label := range labels {
		if avSet.Check(label) {
			toMove = append(toMove, label)
			delete(avSet, label)
		}
	}
	return toMove
}

func (g *Graph) removeAvailabiltySet(av AvailabilitySet) {
	toRemove := av.Nodes()
	for _, n := range toRemove {
		g.removeNode(n)
	}
}

func (g Graph) findAvailabilitySet(label string) AvailabilitySet {
	for _, av := range g.Availability {
		if av.Check(label) {
			return av
		}
	}
	return nil
}

// Merge all placement rules such that each label appears as a target only once
func (g *Graph) addPlacementRule(rule Placement) error {
	if !rule.Exclusive {
		return nil
	}

	targetNodes, sepNodes := validateRule(rule, *g)

	for _, target := range targetNodes {
		for _, sep := range sepNodes {
			if target != sep {
				g.Placement[target] = append(
					[]string{sep},
					g.Placement[target]...,
				)
				g.Placement[sep] = append(g.Placement[sep], target)
			}
		}
	}

	// Add extra rule separating "public" into its own availability set.
	// Remove once "implicit placement rules" are incorporated into dsl.
	if _, ok := g.Nodes[PublicInternetLabel]; ok {
		allLabels := make([]string, 0, len(g.Nodes))
		for _, lab := range g.getNodes() {
			if lab.Name != PublicInternetLabel {
				allLabels = append(allLabels, lab.Name)
				g.Placement[lab.Name] = append(
					g.Placement[lab.Name],
					PublicInternetLabel,
				)
			}
		}
		g.Placement[PublicInternetLabel] = allLabels
	}

	g.placeNodes()
	return nil
}

func validateRule(place Placement, g Graph) ([]string, []string) {
	var targetNodes []string
	var otherNodes []string

	for _, node := range g.Nodes {
		if node.Label == place.TargetLabel {
			targetNodes = append(targetNodes, node.Name)
		}

		if node.Label == place.OtherLabel {
			otherNodes = append(otherNodes, node.Name)
		}
	}

	return targetNodes, otherNodes
}

// Finding minimal number of availability sets is NP-complete.
// Try to pack into as few availability sets as possible.
// For each placement rule:
//   - Find the affected node's availability set.
//   - Check if any restricted nodes are in the availability set.
//   - If so, remove those nodes from the availability set.
//   - For each removed node, try to find another availability set to move it to.
//   - Create a new set for nodes that could not be moved to an existing set.
func (g *Graph) placeNodes() {
	for node, wantExclusives := range g.Placement {
		if _, ok := g.Nodes[node]; !ok {
			panic(
				fmt.Errorf(
					"invalid node: %s, nodes: %s",
					node,
					g.getNodes(),
				),
			)
		}

		av := g.findAvailabilitySet(node)
		if av == nil {
			panic(fmt.Errorf("could not find availabilty set: %s", node))
		}
		toMove := av.removeAndCheck(wantExclusives...)

		for ind, move := range toMove {
			avoids := g.Placement[move]
			for _, avMoveTo := range g.Availability {
				noConflicts := true
				for _, avoid := range avoids {
					if avMoveTo.Check(avoid) {
						noConflicts = false
						break
					}
				}
				if noConflicts {
					avMoveTo.Insert(move)
					if ind+1 >= len(toMove) {
						toMove = toMove[:ind]
					} else {
						toMove = append(toMove[:ind],
							toMove[ind+1:]...)
					}
					break
				}
			}
		}

		if len(toMove) > 0 {
			newAv := make(AvailabilitySet)
			newAv.Insert(toMove...)
			g.Availability = append(g.Availability, newAv)
		}
	}
}
