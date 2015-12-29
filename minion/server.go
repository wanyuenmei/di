package main

import (
	"net"
	"time"

	"github.com/NetSys/di/minion/pb"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

type MServer struct {
	Cfg           pb.MinionConfig
	ConfigChan    chan pb.MinionConfig
	ContainerChan chan pb.ContainerConfig
}

func (s *MServer) GetMinionConfig(cts context.Context,
	_ *pb.Request) (*pb.MinionConfig, error) {
	return &s.Cfg, nil
}

func (s *MServer) SetMinionConfig(ctx context.Context,
	msg *pb.MinionConfig) (*pb.Reply, error) {
	s.Cfg = *msg
	s.ConfigChan <- s.Cfg
	return &pb.Reply{Success: true}, nil
}

func (s *MServer) SetContainerConfig(ctx context.Context,
	msg *pb.ContainerConfig) (*pb.Reply, error) {
	s.ContainerChan <- *msg
	return &pb.Reply{Success: true}, nil
}

func NewMinionServer() MServer {
	cfgChan := make(chan pb.MinionConfig)
	cntrChan := make(chan pb.ContainerConfig)
	mServer := MServer{
		ConfigChan:    cfgChan,
		ContainerChan: cntrChan,
		Cfg:           pb.MinionConfig{"", pb.MinionConfig_NONE, "", ""},
	}

	go func() {
		var sock net.Listener
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
		pb.RegisterMinionServer(s, &mServer)
		s.Serve(sock)
	}()

	return mServer
}
