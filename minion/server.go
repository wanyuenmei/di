package main

import (
	"net"
	"time"

	"github.com/NetSys/di/db"
	"github.com/NetSys/di/minion/pb"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

type server struct {
	db.Conn
}

func minionServerRun(conn db.Conn) {
	var sock net.Listener
	server := server{conn}
	for {
		var err error
		sock, err = net.Listen("tcp", ":9999")
		if err != nil {
			log.Warning("Failed to open socket: %s", err)
		} else {
			break
		}

		time.Sleep(30 * time.Second)
	}

	s := grpc.NewServer()
	pb.RegisterMinionServer(s, server)
	log.Info("Waiting for configuration.")
	s.Serve(sock)
}

func (s server) GetMinionConfig(cts context.Context,
	_ *pb.Request) (*pb.MinionConfig, error) {
	var minionRows []db.Minion
	s.Transact(func(view db.Database) error {
		minionRows = view.SelectFromMinion(nil)
		return nil
	})

	var cfg pb.MinionConfig
	switch len(minionRows) {
	case 0:
		cfg.Role = pb.MinionConfig_Role(db.None)
	case 1:
		m := minionRows[0]
		cfg.ID = m.MinionID
		cfg.Role = pb.MinionConfig_Role(m.Role)
		cfg.PrivateIP = m.PrivateIP
		cfg.EtcdToken = m.EtcdToken
	default:
		panic("Not Reached")
	}

	return &cfg, nil
}

func (s server) SetMinionConfig(ctx context.Context,
	msg *pb.MinionConfig) (*pb.Reply, error) {
	go s.Transact(func(view db.Database) error {
		minionSlice := view.SelectFromMinion(nil)
		var minion db.Minion
		switch len(minionSlice) {
		case 0:
			minion = view.InsertMinion()
		case 1:
			minion = minionSlice[0]
		default:
			panic("Not Reached")
		}

		minion.MinionID = msg.ID
		minion.Role = db.Role(msg.Role)
		minion.PrivateIP = msg.PrivateIP
		minion.EtcdToken = msg.EtcdToken
		view.Commit(minion)

		return nil
	})

	return &pb.Reply{Success: true}, nil
}

func (s server) SetEndpoints(ctx context.Context,
	msg *pb.EndpointList) (*pb.Reply, error) {

	go s.Transact(func(view db.Database) error {
		setEndpointsTxn(view, msg.Endpoints)
		return nil
	})

	return &pb.Reply{Success: true}, nil
}

func setEndpointsTxn(view db.Database, eps__ []*pb.Endpoint) {
	/* This function destructively modifles eps, so copy it to avoid surpring
	 * callers. */
	eps := make([]*pb.Endpoint, 0, len(eps__))
	for _, ep := range eps__ {
		eps = append(eps, ep)
	}

	dbcs := view.SelectFromContainer(nil)
	var changes, removes []db.Container
	if len(dbcs) > len(eps) {
		changes = dbcs[:len(eps)]
		removes = dbcs[len(eps):]
	} else {
		changes = dbcs
	}

	/* For each database container, we want to assign it to the eps which would
	* involve the fewest changes.  To do this, we loop through each EPS and calculate
	* an edit distance. */
	for _, dbc := range changes {
		var bestDistance, bestIndex int
		for i, ep := range eps {
			ed := editDistance(dbc.Labels, ep.Labels)
			if i == 0 || ed < bestDistance {
				bestDistance = ed
				bestIndex = i
			}

			if ed == 0 {
				break
			}
		}
		dbc.Labels = eps[bestIndex].Labels
		view.Commit(dbc)

		//Delete the used element.
		eps[bestIndex] = eps[len(eps)-1]
		eps = eps[:len(eps)-1]
	}

	for _, dbc := range removes {
		view.Remove(dbc)
	}

	for _, ep := range eps {
		dbc := view.InsertContainer()
		dbc.Labels = ep.Labels
		view.Commit(dbc)
	}
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
