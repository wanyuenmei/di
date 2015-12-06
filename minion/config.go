package main

import (
	"net"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/grpc"

	. "github.com/NetSys/di/minion/proto"
)

type configServer struct {
	cfg        MinionConfig
	configChan chan MinionConfig
}

func (s *configServer) SetMinionConfig(ctx context.Context,
	msg *MinionConfig) (*Reply, error) {
	s.cfg = *msg
	s.configChan <- s.cfg
	return &Reply{Success: true}, nil
}

func NewConfigChannel() <-chan MinionConfig {
	cfgChan := make(chan MinionConfig)

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
		RegisterMinionServer(s, &configServer{
			configChan: cfgChan,
			cfg:        MinionConfig{"", MinionConfig_NONE, "", ""},
		})
		s.Serve(sock)
	}()

	return cfgChan
}
