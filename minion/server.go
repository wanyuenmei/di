package main

import (
	"net"
	"sort"
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
			log.Info("Received initial configuation")
			minion = view.InsertMinion()
		case 1:
			minion = minionSlice[0]
		default:
			panic("Not Reached")
		}

		minion.MinionID = msg.ID
		minion.Role = db.Role(msg.Role)
		minion.PrivateIP = msg.PrivateIP
		view.Commit(minion)

		updatePolicy(view, msg.Spec)

		return nil
	})

	return &pb.Reply{Success: true}, nil
}

func (s server) BootEtcd(ctx context.Context,
	members *pb.EtcdMembers) (*pb.Reply, error) {
	go s.Transact(func(view db.Database) error {
		minionSlice := view.SelectFromMinion(nil)
		var minion db.Minion
		switch len(minionSlice) {
		case 0:
			log.Info("Received initial boot etcd request")
			minion = view.InsertMinion()
		case 1:
			minion = minionSlice[0]
		default:
			panic("Not Reached")
		}

		minion.EtcdIPs = members.IPs
		view.Commit(minion)

		return nil
	})

	return &pb.Reply{Success: true}, nil
}
