package db

//The Role within the cluster each machine assumes.
import (
	"errors"

	"github.com/NetSys/di/minion/pb"
)

// The Role a machine may take on within the cluster.
type Role int

const (
	None Role = iota
	Worker
	Master
)

func (r Role) String() string {
	switch r {
	case None:
		return ""
	case Worker:
		return "Worker"
	case Master:
		return "Master"
	default:
		panic("Not Reached")
	}
}

func RoleToPB(r Role) pb.MinionConfig_Role {
	switch r {
	case None:
		return pb.MinionConfig_NONE
	case Worker:
		return pb.MinionConfig_WORKER
	case Master:
		return pb.MinionConfig_MASTER
	default:
		panic("Not Reached")
	}
}

func PBToRole(p pb.MinionConfig_Role) Role {
	switch p {
	case pb.MinionConfig_NONE:
		return None
	case pb.MinionConfig_WORKER:
		return Worker
	case pb.MinionConfig_MASTER:
		return Master
	default:
		panic("Not Reached")
	}
}

// A Provider implements a cloud interface on which machines may be instantiated.
type Provider string

const (
	// AmazonSpot runs spot requests on Amazon EC2.
	AmazonSpot Provider = "AmazonSpot"
	Google              = "Google"
	Vagrant             = "Vagrant"
	Azure               = "Azure"
)

// ParseProvider returns the Provider represented by 'name' or an error.
func ParseProvider(name string) (Provider, error) {
	switch name {
	case "AmazonSpot", "Google", "Vagrant", "Azure":
		return Provider(name), nil
	default:
		return "", errors.New("unknown provider")
	}
}
