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
		updatePlacements(view, compiled)
		// The container table is aspirational -- it's the set of containers that
		// should exist.  In the workers, however, the container table is just
		// what's running locally.  That's why we only sync the database
		// containers on the master.
		updateContainers(view, compiled)
	}
	updateConnections(view, compiled)
}

func toDBPlacements(dslPlacements []dsl.Placement) []db.Placement {
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

	var placements []db.Placement
	for p := range placementSet {
		placements = append(placements, p)
	}
	return placements
}

func updatePlacements(view db.Database, spec dsl.Dsl) {
	dslPlacements := toDBPlacements(spec.QueryPlacements())

	scoreFunc := func(left, right interface{}) int {
		wantedPlacement := left.(db.Placement)
		havePlacement := right.(db.Placement)

		if wantedPlacement.TargetLabel == havePlacement.TargetLabel &&
			wantedPlacement.Rule == havePlacement.Rule {
			return 0
		}
		return -1
	}

	_, addSet, removeSet := join.Join(dslPlacements, view.SelectFromPlacement(nil), scoreFunc)

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
