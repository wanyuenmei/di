package main

import (
    "fmt"
    "net"

    pb "github.com/NetSys/di/minion/rpc"

    "golang.org/x/net/context"
    "google.golang.org/grpc"
    "github.com/op/go-logging"
)

var log = logging.MustGetLogger("minion")

type server struct {
    name string
    role string
}

func (s *server) Boot(ctx context.Context, msg *pb.BootRequest) (*pb.BootReply,
error) {
    if s.role == "master" {
        log.Warning("Received boot command, but am master")
        return &pb.BootReply{Success: false}, nil
    }
    s.name = msg.Name
    s.role = msg.Role
    log.Info(fmt.Sprintf("Set name: %s\n", msg.Name))
    log.Info(fmt.Sprintf("Set role: %s\n", msg.Role))
    return &pb.BootReply{Success: true}, nil
}

func main() {
    sock, e := net.Listen("tcp", ":8080")
    if e != nil {
        log.Fatal(e)
    }
    s := grpc.NewServer()
    pb.RegisterDiMinionServer(s, &server{})
    s.Serve(sock)
}
