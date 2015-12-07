package main

import (
	"net"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/grpc"

	. "github.com/NetSys/di/minion/proto"
)

type MServer struct {
	Cfg           MinionConfig
	ConfigChan    chan MinionConfig
	ContainerChan chan ContainerConfig
}

func (s *MServer) GetMinionConfig(cts context.Context,
	_ *Request) (*MinionConfig, error) {
	return &s.Cfg, nil
}

func (s *MServer) SetMinionConfig(ctx context.Context,
	msg *MinionConfig) (*Reply, error) {
	s.Cfg = *msg
	s.ConfigChan <- s.Cfg
	return &Reply{Success: true}, nil
}

func (s *MServer) SetContainerConfig(ctx context.Context,
	msg *ContainerConfig) (*Reply, error) {
	s.ContainerChan <- *msg
	return &Reply{Success: true}, nil
}

func NewMinionServer() MServer {
	cfgChan := make(chan MinionConfig)
	cntrChan := make(chan ContainerConfig)
	mServer := MServer{
		ConfigChan:    cfgChan,
		ContainerChan: cntrChan,
		Cfg:           MinionConfig{"", MinionConfig_NONE, "", ""},
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
		RegisterMinionServer(s, &mServer)
		s.Serve(sock)
	}()

	return mServer
}
