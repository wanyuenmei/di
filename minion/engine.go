package main

import (
	"sort"
	"strings"
	"text/scanner"

	"github.com/NetSys/quilt/db"
	"github.com/NetSys/quilt/join"
	"github.com/NetSys/quilt/stitch"
	"github.com/NetSys/quilt/util"

	log "github.com/Sirupsen/logrus"
)

func updatePolicy(view db.Database, role db.Role, spec string) {
	var sc scanner.Scanner
	compiled, err := stitch.New(*sc.Init(strings.NewReader(spec)), "")
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

func toDBPlacements(stitchPlacements []stitch.Placement) db.PlacementSlice {
	placementSet := make(map[db.Placement]struct{})
	for _, rule := range stitchPlacements {
		if rule.OtherLabel != "" {
			placement := db.Placement{
				TargetLabel: rule.TargetLabel,
				Rule: db.LabelRule{
					OtherLabel: rule.OtherLabel,
					Exclusive:  rule.Exclusive,
				},
			}
			placementSet[placement] = struct{}{}
		}

		// XXX: This is somewhat of a hack to avoid having to change
		// db/placement.go.  In a future patch, when we ditch docker swarm, a
		// much cleaner permanent solution is comint.
		machineAttributes := map[string]string{
			"provider": rule.Provider,
			"size":     rule.Size,
			"region":   rule.Region,
		}

		for attr, val := range machineAttributes {
			if val == "" {
				continue
			}
			placement := db.Placement{
				TargetLabel: rule.TargetLabel,
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

func updatePlacements(view db.Database, spec stitch.Stitch) {
	stitchPlacements := toDBPlacements(spec.QueryPlacements())
	key := func(val interface{}) interface{} {
		pVal := val.(db.Placement)
		return struct {
			tl   string
			rule db.PlacementRule
		}{pVal.TargetLabel, pVal.Rule}
	}

	_, addSet, removeSet := join.HashJoin(stitchPlacements,
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

func updateConnections(view db.Database, spec stitch.Stitch) {
	scs, vcs := stitch.ConnectionSlice(spec.QueryConnections()),
		view.SelectFromConnection(nil)

	dbcKey := func(val interface{}) interface{} {
		c := val.(db.Connection)
		return stitch.Connection{
			From:    c.From,
			To:      c.To,
			MinPort: c.MinPort,
			MaxPort: c.MaxPort,
		}
	}

	pairs, stitchs, dbcs := join.HashJoin(scs, db.ConnectionSlice(vcs), nil, dbcKey)

	for _, dbc := range dbcs {
		view.Remove(dbc.(db.Connection))
	}

	for _, stitchc := range stitchs {
		pairs = append(pairs, join.Pair{L: stitchc, R: view.InsertConnection()})
	}

	for _, pair := range pairs {
		stitchc := pair.L.(stitch.Connection)
		dbc := pair.R.(db.Connection)

		dbc.From = stitchc.From
		dbc.To = stitchc.To
		dbc.MinPort = stitchc.MinPort
		dbc.MaxPort = stitchc.MaxPort
		view.Commit(dbc)
	}
}

func queryContainers(spec stitch.Stitch) []db.Container {
	containers := map[int]*db.Container{}
	for _, c := range spec.QueryContainers() {
		containers[c.ID] = &db.Container{
			Command: c.Command,
			Image:   c.Image,
			Env:     c.Env,
		}
	}

	for label, ids := range spec.QueryLabels() {
		for _, id := range ids {
			containers[id].Labels = append(containers[id].Labels, label)
		}
	}

	var ret []db.Container
	for _, c := range containers {
		ret = append(ret, *c)
	}

	return ret
}

func updateContainers(view db.Database, spec stitch.Stitch) {
	score := func(l, r interface{}) int {
		left := l.(db.Container)
		right := r.(db.Container)

		if left.Image != right.Image ||
			!util.StrSliceEqual(left.Command, right.Command) ||
			!util.StrStrMapEqual(left.Env, right.Env) {
			return -1
		}

		return util.EditDistance(left.Labels, right.Labels)
	}

	pairs, news, dbcs := join.Join(queryContainers(spec),
		view.SelectFromContainer(nil), score)

	for _, dbc := range dbcs {
		view.Remove(dbc.(db.Container))
	}

	for _, new := range news {
		pairs = append(pairs, join.Pair{L: new, R: view.InsertContainer()})
	}

	for _, pair := range pairs {
		newc := pair.L.(db.Container)
		dbc := pair.R.(db.Container)

		// By sorting the labels we prevent the database from getting confused
		// when their order is non determinisitic.
		dbc.Labels = newc.Labels
		sort.Sort(sort.StringSlice(dbc.Labels))

		dbc.Command = newc.Command
		dbc.Image = newc.Image
		dbc.Env = newc.Env
		view.Commit(dbc)
	}
}
