package main

import (
	"net"

	"golang.org/x/net/context"
	"google.golang.org/grpc"

	"github.com/NetSys/di/minion/proto"
)

type configServer struct {
	cfg        proto.MinionConfig
	configChan chan proto.MinionConfig
	booted     bool
}

func (s *configServer) SetMinionConfig(ctx context.Context,
	msg *proto.MinionConfig) (*proto.Reply, error) {
	if s.booted {
		if *msg != s.cfg {
			return &proto.Reply{Success: false,
				Error: "Cannot change MinionConfig"}, nil
		}
		return &proto.Reply{Success: true}, nil
	}

	s.booted = true
	s.cfg = *msg
	s.configChan <- s.cfg
	return &proto.Reply{Success: true}, nil
}

func NewConfigChannel() (chan proto.MinionConfig, error) {
	cfgChan := make(chan proto.MinionConfig)
	server := configServer{configChan: cfgChan}

	sock, err := net.Listen("tcp", ":8080")
	if err != nil {
		return cfgChan, err
	}

	s := grpc.NewServer()
	proto.RegisterMinionServer(s, &server)
	go s.Serve(sock)

	return cfgChan, nil
}
