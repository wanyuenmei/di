package provider

import "github.com/NetSys/di/db"

// Machine represents an instance of a machine booted by a Provider.
type Machine struct {
	ID        string
	PublicIP  string
	PrivateIP string
}

// Provider defines an interface for interacting with cloud providers.
type Provider interface {
	Start(conn db.Conn, id int, namespace string, keys []string) error

	Get() ([]Machine, error)

	Boot(count int) error

	Stop(ids []string) error

	Disconnect()
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
