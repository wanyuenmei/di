package main

import (
	"reflect"
	"strings"

	"github.com/NetSys/di/db"
	"github.com/NetSys/di/dsl"
)

func updatePolicy(view db.Database, spec string) {
	compiled, err := dsl.New(strings.NewReader(spec))
	if err != nil {
		log.Warning("Invalid spec: %s", err)
		return
	}

	updateContainers(view, compiled)
	updateConnections(view, compiled)
}

func updateConnections(view db.Database, spec dsl.Dsl) {
	dslSlice := spec.QueryConnections()

	dslMap := make(map[dsl.Connection]struct{})
	for _, c := range dslSlice {
		dslMap[c] = struct{}{}
	}

	for _, dbc := range view.SelectFromConnection(nil) {
		dslC := dsl.Connection{
			From:    dbc.From,
			To:      dbc.To,
			MinPort: dbc.MinPort,
			MaxPort: dbc.MaxPort,
		}

		if _, ok := dslMap[dslC]; ok {
			delete(dslMap, dslC)
			continue
		} else {
			view.Remove(dbc)
		}
	}

	for dslc := range dslMap {
		dbc := view.InsertConnection()
		dbc.From = dslc.From
		dbc.To = dslc.To
		dbc.MinPort = dslc.MinPort
		dbc.MaxPort = dslc.MaxPort
		view.Commit(dbc)
	}
}

func updateContainers(view db.Database, spec dsl.Dsl) {
	dslSlice := spec.QueryContainers()
	dbSlice := view.SelectFromContainer(nil)

	for _, dbc := range dbSlice {
		var best *dsl.Container
		dslSlice, best = bestFit(dbc, dslSlice)
		if best == nil {
			view.Remove(dbc)
			continue
		}

		dbc.Image = best.Image
		dbc.Command = best.Command
		dbc.Labels = best.Labels
		view.Commit(dbc)
	}

	for _, dslc := range dslSlice {
		dbc := view.InsertContainer()
		dbc.Labels = dslc.Labels
		dbc.Command = dslc.Command
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
		if dbc.Image != dslc.Image ||
			!reflect.DeepEqual(dbc.Command, dslc.Command) {
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
