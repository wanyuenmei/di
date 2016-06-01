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

	var cfg pb.MinionConfig

	if m, err := s.MinionSelf(); err != nil {
		cfg.Role = pb.MinionConfig_Role(m.Role)
		cfg.PrivateIP = m.PrivateIP
		cfg.Spec = m.Spec
		cfg.Provider = m.Provider
		cfg.Size = m.Size
		cfg.Region = m.Region
	} else {
		cfg.Role = pb.MinionConfig_Role(db.None)
	}

	return &cfg, nil
}

func (s server) SetMinionConfig(ctx context.Context,
	msg *pb.MinionConfig) (*pb.Reply, error) {
	go s.Transact(func(view db.Database) error {
		minion, err := view.MinionSelf()
		if err != nil {
			log.Info("Received initial configuation.")
			minion = view.InsertMinion()
		}

		minion.Role = db.Role(msg.Role)
		minion.PrivateIP = msg.PrivateIP
		minion.Spec = msg.Spec
		minion.Provider = msg.Provider
		minion.Size = msg.Size
		minion.Region = msg.Region
		minion.Self = true
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
