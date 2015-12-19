//go:generate stringer -type=Role -type=Provider

package db

//The Role within the cluster each machine assumes.
import (
	"fmt"

	"github.com/NetSys/di/minion/pb"
)

// The Role a machine may take on within the cluster.
type Role pb.MinionConfig_Role

const (
	// None machines haven't had a role assigned yet.
	None Role = Role(pb.MinionConfig_NONE)

	// Worker machines are responsible for running containers.
	Worker = Role(pb.MinionConfig_WORKER)

	// Master machines are responsible for running control processes.
	Master = Role(pb.MinionConfig_MASTER)
)

// A Provider implements a cloud interface on which machines may be instantiated.
type Provider int

const (
	// AmazonSpot runs spot requests on Amazon EC2.
	AmazonSpot Provider = iota
)

// ParseProvider returns the Provider represented by 'name' or an error.
func ParseProvider(name string) (Provider, error) {
	switch name {
	case "AmazonSpot":
		return AmazonSpot, nil
	default:
		return 0, fmt.Errorf("Unknown provider: %s", name)
	}
}
