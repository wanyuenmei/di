package provider

import "github.com/NetSys/di/db"

type Machine struct {
	ID        string
	PublicIP  string
	PrivateIP string
}

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
