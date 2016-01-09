package main

import (
	"strings"

	"github.com/NetSys/di/db"
	"github.com/NetSys/di/dsl"
)

func UpdatePolicy(view db.Database, spec string) {
	compiled, err := dsl.New(strings.NewReader(spec))
	if err != nil {
		log.Warning("Invalid spec: %s", err)
		return
	}

	dslSlice := compiled.QueryContainers()
	dbSlice := view.SelectFromContainer(nil)

	for _, dbc := range dbSlice {
		var best *dsl.Container
		dslSlice, best = bestFit(dbc, dslSlice)
		if best == nil {
			view.Remove(dbc)
			continue
		}

		dbc.Image = best.Image
		dbc.Labels = best.Labels
		view.Commit(dbc)
	}

	for _, dslc := range dslSlice {
		dbc := view.InsertContainer()
		dbc.Labels = dslc.Labels
		dbc.Image = dslc.Image
		view.Commit(dbc)
	}
}

// Find the best fit for 'c' in 'dslSlice' and return it along with an update 'dslSlice'
func bestFit(dbc db.Container, dslSlice []*dsl.Container) ([]*dsl.Container,
	*dsl.Container) {

	bestIndex := -1
	bestDistance := -1
	for i, dslc := range dslSlice {
		if dbc.Image != dslc.Image {
			continue
		}

		ed := editDistance(dbc.Labels, dslc.Labels)
		if bestIndex < 0 || ed < bestDistance {
			bestDistance = ed
			bestIndex = i
		}

		if ed == 0 {
			break
		}
	}

	if bestIndex < 0 {
		return dslSlice, nil
	}

	best := dslSlice[bestIndex]
	dslSlice[bestIndex] = dslSlice[len(dslSlice)-1]
	dslSlice = dslSlice[:len(dslSlice)-1]
	return dslSlice, best
}

func editDistance(a, b []string) int {
	amap := make(map[string]struct{})

	for _, label := range a {
		amap[label] = struct{}{}
	}

	ed := 0
	for _, label := range b {
		if _, ok := amap[label]; ok {
			delete(amap, label)
		} else {
			ed++
		}
	}

	ed += len(amap)
	return ed
}
