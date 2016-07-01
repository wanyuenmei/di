package main

import (
	"fmt"

	"github.com/NetSys/quilt/stitch"
)

// A set of containers which can be placed together on a VM.
type availabilitySet map[string]struct{}

func (avSet availabilitySet) nodes() []string {
	var labels []string
	for l := range avSet {
		labels = append(labels, l)
	}
	return labels
}

func (avSet availabilitySet) str() string {
	return fmt.Sprint(avSet.nodes())
}

func (avSet availabilitySet) check(label string) bool {
	_, ok := avSet[label]
	return ok
}

func (avSet availabilitySet) copyAvSet() availabilitySet {
	newAvSet := map[string]struct{}{}
	for label := range avSet {
		newAvSet[label] = struct{}{}
	}
	return newAvSet
}

func (avSet availabilitySet) insert(labels ...string) {
	for _, label := range labels {
		avSet[label] = struct{}{}
	}
}

func (avSet availabilitySet) remove(labels ...string) {
	for _, label := range labels {
		delete(avSet, label)
	}
}

func (avSet availabilitySet) removeAndCheck(labels ...string) []string {
	var toMove []string
	for _, label := range labels {
		if avSet.check(label) {
			toMove = append(toMove, label)
			delete(avSet, label)
		}
	}
	return toMove
}

// Merge all placement rules such that each label appears as a target only once
func (g *graph) addPlacementRule(rule stitch.Placement) {
	if !rule.Exclusive {
		return
	}

	target, sep := validateRule(rule, *g)
	g.placement[target] = append([]string{sep}, g.placement[target]...)
	g.placement[sep] = append(g.placement[sep], target)

	// Add extra rule separating "public" into its own availability set.
	// Remove once "implicit placement rules" are incorporated into dsl.
	if _, ok := g.nodes[stitch.PublicInternetLabel]; ok {
		allLabels := make([]string, 0, len(g.nodes))
		for _, lab := range g.getNodes() {
			if lab.name != stitch.PublicInternetLabel {
				allLabels = append(allLabels, lab.name)
				g.placement[lab.name] = append(g.placement[lab.name], stitch.PublicInternetLabel)
			}
		}
		g.placement[stitch.PublicInternetLabel] = allLabels
	}

	g.placeNodes()
}

func validateRule(place stitch.Placement, g graph) (string, string) {
	targetNode, ok := g.nodes[place.TargetLabel]
	if !ok {
		panic(fmt.Errorf("placement constraint: node not found: %s", place.TargetLabel))
	}

	var wantExclusive string
	if place.OtherLabel != "" {
		other, ok := g.nodes[place.OtherLabel]
		if !ok {
			panic(fmt.Errorf("placement constraint: node not found: %s", other))
		}

		if other.name != targetNode.name {
			wantExclusive = other.name
		}
	}

	return targetNode.name, wantExclusive
}

func (g *graph) removeAvailabiltySet(av availabilitySet) {
	toRemove := av.nodes()
	for _, n := range toRemove {
		g.removeNode(n)
	}
}

func (g graph) findAvailabilitySet(label string) availabilitySet {
	for _, av := range g.availability {
		if av.check(label) {
			return av
		}
	}
	return nil
}

// Finding minimal number of availability sets is NP-complete.
// Try to pack into as few availability sets as possible.
// For each placement rule:
//   - Find the affected node's availability set.
//   - Check if any restricted nodes are in the availability set.
//   - If so, remove those nodes from the availability set.
//   - For each removed node, try to find another availability set to move it to.
//   - Create a new set for nodes that could not be moved to an existing set.
func (g *graph) placeNodes() {
	for node, wantExclusives := range g.placement {
		av := g.findAvailabilitySet(node)
		if av == nil {
			panic(fmt.Errorf("could not find availabilty set: %s", node))
		}
		toMove := av.removeAndCheck(wantExclusives...)

		for ind, move := range toMove {
			avoids := g.placement[move]
			for _, avMoveTo := range g.availability {
				noConflicts := true
				for _, avoid := range avoids {
					if avMoveTo.check(avoid) {
						noConflicts = false
						break
					}
				}
				if noConflicts {
					avMoveTo.insert(move)
					if ind+1 >= len(toMove) {
						toMove = toMove[:ind]
					} else {
						toMove = append(toMove[:ind], toMove[ind+1:]...)
					}
					break
				}
			}
		}

		if len(toMove) > 0 {
			newAv := make(availabilitySet)
			newAv.insert(toMove...)
			g.availability = append(g.availability, newAv)
		}
	}
}
