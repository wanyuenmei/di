package provider

import (
	"github.com/NetSys/di/db"
	"github.com/NetSys/di/dsl"
)

// Machine represents an instance of a machine booted by a Provider.
type Machine struct {
	ID        string
	PublicIP  string
	PrivateIP string
	Size      string
	Provider  db.Provider
}

// Provider defines an interface for interacting with cloud providers.
type Provider interface {
	Start(conn db.Conn, id int, namespace string, keys []string) error

	Get() ([]Machine, error)

	Boot(bootSet []Machine) error

	Stop(ids []string) error

	Disconnect()

	PickBestSize(ram dsl.Range, cpu dsl.Range, maxPrice float64) string
}

// New returns an empty instance of the Provider represented by `dbp`
func New(dbp db.Provider) Provider {
	switch dbp {
	case db.AmazonSpot:
		return &awsSpotCluster{}
	case db.Google:
		return &gceCluster{}
	case db.Azure:
		return &azureCluster{}
	case db.Vagrant:
		return &vagrantCluster{}
	default:
		panic("Unimplemented")
	}
}

// GroupBy transforms the `machines` into a map of `db.Provider` to the machines
// with that provider.
func GroupBy(machines []Machine) map[db.Provider][]Machine {
	machineMap := make(map[db.Provider][]Machine)
	for _, m := range machines {
		if _, ok := machineMap[m.Provider]; !ok {
			machineMap[m.Provider] = []Machine{}
		}
		machineMap[m.Provider] = append(machineMap[m.Provider], m)
	}

	return machineMap
}

// IDs returns the cloud IDs of `machines`.
func IDs(machines []Machine) []string {
	var IDs []string
	for _, m := range machines {
		IDs = append(IDs, m.ID)
	}
	return IDs
}
