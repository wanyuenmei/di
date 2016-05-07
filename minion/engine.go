package main

import (
	"reflect"
	"sort"
	"strings"
	"text/scanner"

	"github.com/NetSys/di/db"
	"github.com/NetSys/di/dsl"
	"github.com/NetSys/di/join"
	"github.com/NetSys/di/util"

	log "github.com/Sirupsen/logrus"
)

func updatePolicy(view db.Database, role db.Role, spec string) {
	var sc scanner.Scanner
	compiled, err := dsl.New(*sc.Init(strings.NewReader(spec)), []string{})
	if err != nil {
		log.WithError(err).Warn("Invalid spec.")
		return
	}

	if role == db.Master {
		// The container table is aspirational -- it's the set of containers that
		// should exist.  In the workers, however, the container table is just
		// what's running locally.  That's why we only sync the database
		// containers on the master.
		updateContainers(view, compiled)
	}
	updateConnections(view, compiled)
}

func updateConnections(view db.Database, spec dsl.Dsl) {
	dslcs := spec.QueryConnections()
	for _, dbc := range view.SelectFromConnection(nil) {
		key := dsl.Connection{
			From:    dbc.From,
			To:      dbc.To,
			MinPort: dbc.MinPort,
			MaxPort: dbc.MaxPort,
		}

		if _, ok := dslcs[key]; ok {
			delete(dslcs, key)
			continue
		}

		view.Remove(dbc)
	}

	for dslc := range dslcs {
		dbc := view.InsertConnection()
		dbc.From = dslc.From
		dbc.To = dslc.To
		dbc.MinPort = dslc.MinPort
		dbc.MaxPort = dslc.MaxPort
		view.Commit(dbc)
	}
}

func updateContainers(view db.Database, spec dsl.Dsl) {
	score := func(l, r interface{}) int {
		dslc := l.(*dsl.Container)
		dbc := r.(db.Container)

		if dbc.Image != dslc.Image ||
			!reflect.DeepEqual(dbc.Command, dslc.Command) {
			return -1
		}

		score := util.EditDistance(dbc.Labels, dslc.Labels())
		for k := range dbc.Placement.Exclusive {
			if _, ok := dslc.Placement.Exclusive[k]; !ok {
				score += 100
			}
		}

		for k := range dslc.Placement.Exclusive {
			if _, ok := dbc.Placement.Exclusive[k]; !ok {
				score += 100
			}
		}

		for k, v := range dbc.Env {
			v2 := dslc.Env[k]
			if v != v2 {
				return -1
			}
		}

		return score
	}

	pairs, dsls, dbcs := join.Join(spec.QueryContainers(),
		view.SelectFromContainer(nil), score)

	for _, dbc := range dbcs {
		view.Remove(dbc.(db.Container))
	}

	for _, dslc := range dsls {
		pairs = append(pairs, join.Pair{L: dslc, R: view.InsertContainer()})
	}

	for _, pair := range pairs {
		dslc := pair.L.(*dsl.Container)
		dbc := pair.R.(db.Container)

		// By sorting the labels we prevent the database from getting confused
		// when their order is non determinisitic.
		dbc.Labels = dslc.Labels()
		sort.Sort(sort.StringSlice(dbc.Labels))

		dbc.Command = dslc.Command
		dbc.Image = dslc.Image
		dbc.Placement.Exclusive = dslc.Placement.Exclusive
		dbc.Env = dslc.Env
		view.Commit(dbc)
	}
}
