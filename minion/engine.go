package main

import (
	"reflect"
	"sort"
	"strings"
	"text/scanner"

	"github.com/NetSys/quilt/db"
	"github.com/NetSys/quilt/dsl"
	"github.com/NetSys/quilt/join"
	"github.com/NetSys/quilt/util"

	log "github.com/Sirupsen/logrus"
)

func updatePolicy(view db.Database, role db.Role, spec string) {
	var sc scanner.Scanner
	compiled, err := dsl.New(*sc.Init(strings.NewReader(spec)), []string{})
	if err != nil {
		log.WithError(err).Warn("Invalid spec.")
		return
	}

	updateConnections(view, compiled)
	if role == db.Master {
		// This must happen after `updateConnections` because we generate
		// placement rules based on whether there are incoming connections from
		// public internet.
		updatePlacements(view, compiled)

		// The container table is aspirational -- it's the set of containers that
		// should exist.  In the workers, however, the container table is just
		// what's running locally.  That's why we only sync the database
		// containers on the master.
		updateContainers(view, compiled)
	}
}

func toDBPlacements(dslPlacements []dsl.Placement) db.PlacementSlice {
	placementSet := make(map[db.Placement]struct{})
	for _, dslP := range dslPlacements {
		rule := dslP.Rule
		for _, label := range rule.OtherLabels {
			placement := db.Placement{
				TargetLabel: dslP.TargetLabel,
				Rule: db.LabelRule{
					OtherLabel: label,
					Exclusive:  rule.Exclusive,
				},
			}
			placementSet[placement] = struct{}{}
		}
		for attr, val := range rule.MachineAttributes {
			if val == "" {
				continue
			}
			placement := db.Placement{
				TargetLabel: dslP.TargetLabel,
				Rule: db.MachineRule{
					Exclusive: rule.Exclusive,
					Attribute: attr,
					Value:     val,
				},
			}
			placementSet[placement] = struct{}{}
		}
	}

	var placements db.PlacementSlice
	for p := range placementSet {
		placements = append(placements, p)
	}
	return placements
}

func makeConnectionPlacements(conns []db.Connection) db.PlacementSlice {
	var dbPlacements db.PlacementSlice
	for _, conn := range conns {
		if conn.From == dsl.PublicInternetLabel {
			for p := conn.MinPort; p <= conn.MaxPort; p += 1 {
				dbPlacements = append(dbPlacements, db.Placement{
					TargetLabel: conn.To,
					Rule: db.PortRule{
						Port: p,
					},
				})
			}
		}
	}

	return dbPlacements
}

func updatePlacements(view db.Database, spec dsl.Dsl) {
	dslPlacements := toDBPlacements(spec.QueryPlacements())
	connPlacements := makeConnectionPlacements(view.SelectFromConnection(nil))
	key := func(val interface{}) interface{} {
		pVal := val.(db.Placement)
		return struct {
			tl   string
			rule db.PlacementRule
		}{pVal.TargetLabel, pVal.Rule}
	}

	_, addSet, removeSet := join.HashJoin(append(dslPlacements, connPlacements...),
		db.PlacementSlice(view.SelectFromPlacement(nil)), key, key)

	for _, toAddIntf := range addSet {
		toAdd := toAddIntf.(db.Placement)

		newPlacement := view.InsertPlacement()
		newPlacement.TargetLabel = toAdd.TargetLabel
		newPlacement.Rule = toAdd.Rule
		view.Commit(newPlacement)
	}

	for _, toRemove := range removeSet {
		view.Remove(toRemove.(db.Placement))
	}
}

func makeConnectionSlice(cm map[dsl.Connection]struct{}) dsl.ConnectionSlice {
	slice := make([]dsl.Connection, 0, len(cm))
	for k := range cm {
		slice = append(slice, k)
	}
	return dsl.ConnectionSlice(slice)
}

func updateConnections(view db.Database, spec dsl.Dsl) {
	scs, vcs := makeConnectionSlice(spec.QueryConnections()), view.SelectFromConnection(nil)

	dbcKey := func(val interface{}) interface{} {
		c := val.(db.Connection)
		return dsl.Connection{
			From:    c.From,
			To:      c.To,
			MinPort: c.MinPort,
			MaxPort: c.MaxPort,
		}
	}

	pairs, dsls, dbcs := join.HashJoin(scs, db.ConnectionSlice(vcs), nil, dbcKey)

	for _, dbc := range dbcs {
		view.Remove(dbc.(db.Connection))
	}

	for _, dslc := range dsls {
		pairs = append(pairs, join.Pair{L: dslc, R: view.InsertConnection()})
	}

	for _, pair := range pairs {
		dslc := pair.L.(dsl.Connection)
		dbc := pair.R.(db.Connection)

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
		dbc.Env = dslc.Env
		view.Commit(dbc)
	}
}
