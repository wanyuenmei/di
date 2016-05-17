package main

import (
	"net"
	"sort"
	"time"

	"github.com/NetSys/quilt/db"
	"github.com/NetSys/quilt/minion/pb"

	"golang.org/x/net/context"
	"google.golang.org/grpc"

	log "github.com/Sirupsen/logrus"
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
			log.WithError(err).Error("Failed to open socket.")
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
		cfg.Spec = m.Spec
		cfg.Provider = m.Provider
		cfg.Size = m.Size
		cfg.Region = m.Region
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
			log.Info("Received initial configuation.")
			minion = view.InsertMinion()
		case 1:
			minion = minionSlice[0]
		default:
			panic("Not Reached")
		}

		minion.MinionID = msg.ID
		minion.Role = db.Role(msg.Role)
		minion.PrivateIP = msg.PrivateIP
		minion.Spec = msg.Spec
		minion.Provider = msg.Provider
		minion.Size = msg.Size
		minion.Region = msg.Region
		view.Commit(minion)

		return nil
	})

	return &pb.Reply{Success: true}, nil
}

func (s server) BootEtcd(ctx context.Context,
	members *pb.EtcdMembers) (*pb.Reply, error) {
	go s.Transact(func(view db.Database) error {
		etcdSlice := view.SelectFromEtcd(nil)
		var etcdRow db.Etcd
		switch len(etcdSlice) {
		case 0:
			log.Info("Received boot etcd request.")
			etcdRow = view.InsertEtcd()
		case 1:
			etcdRow = etcdSlice[0]
		default:
			panic("Not Reached")
		}

		etcdRow.EtcdIPs = members.IPs
		sort.Strings(etcdRow.EtcdIPs)
		view.Commit(etcdRow)

		return nil
	})

	return &pb.Reply{Success: true}, nil
}
