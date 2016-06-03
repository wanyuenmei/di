package db

//The Role within the cluster each machine assumes.
import (
	"errors"

	"github.com/NetSys/quilt/minion/pb"
)

// The Role a machine may take on within the cluster.
type Role string

const (
	// None is for workers who haven't been assigned a role yet.
	None Role = ""

	// Worker minions run application containers.
	Worker = "Worker"

	// Master containers provide services for the Worker containers.
	Master = "Master"
)

// RoleToPB converts db.Role to a protobuf role.
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

// PBToRole converts a protobuf role to a db.Role.
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
	// Amazon implements amazon EC2.
	Amazon Provider = "Amazon"

	// Google implements Google Cloud Engine.
	Google = "Google"

	// Vagrant implements local virtual machines.
	Vagrant = "Vagrant"

	// Azure implements the Azure cloud provider.
	Azure = "Azure"
)

// ParseProvider returns the Provider represented by 'name' or an error.
func ParseProvider(name string) (Provider, error) {
	switch name {
	case "Amazon", "Google", "Vagrant", "Azure":
		return Provider(name), nil
	default:
		return "", errors.New("unknown provider")
	}
}

// ParseRole returns the Role represented by the string 'role', or an error.
func ParseRole(role string) (Role, error) {
	switch role {
	case "Master":
		return Master, nil
	case "Worker":
		return Worker, nil
	case "":
		return None, nil
	default:
		return None, errors.New("unknown role")
	}
}

// ProviderSlice is an alias for []Provider to allow for joins
type ProviderSlice []Provider

// Get returns the value contained at the given index
func (ps ProviderSlice) Get(ii int) interface{} {
	return ps[ii]
}

// Len returns the number of items in the slice
func (ps ProviderSlice) Len() int {
	return len(ps)
}
