package provider

import (
	"sync"

	"github.com/NetSys/di/db"
	"github.com/NetSys/di/dsl"
	log "github.com/Sirupsen/logrus"
	"github.com/satori/go.uuid"
)

type vagrantCluster struct {
	namespace string
	vagrant   vagrantAPI
}

func (clst *vagrantCluster) Start(conn db.Conn, clusterID int, namespace string) error {
	vagrant := newVagrantAPI()
	err := vagrant.AddBox("boxcutter/ubuntu1504", "virtualbox")
	if err != nil {
		return err
	}
	clst.namespace = namespace
	clst.vagrant = vagrant
	return nil
}

func (clst vagrantCluster) Boot(bootSet []Machine) error {
	vagrant := clst.vagrant
	var wg sync.WaitGroup
	wg.Add(len(bootSet))
	for _, m := range bootSet {
		id := uuid.NewV4().String()
		err := vagrant.Init(cloudConfigUbuntu(m.SSHKeys, "vivid"), m.Size, id)
		if err != nil {
			vagrant.Destroy(id)
			return err
		}
		go func() {
			defer wg.Done()
			err := vagrant.Up(id)
			if err != nil {
				vagrant.Destroy(id)
			}
		}()
	}
	wg.Wait()
	return nil
}

func (clst vagrantCluster) Get() ([]Machine, error) {
	vagrant := clst.vagrant
	machines := []Machine{}
	instanceIDs, err := vagrant.List()

	if err != nil {
		return machines, err
	} else if len(instanceIDs) == 0 {
		return machines, nil
	}

	for _, instanceID := range instanceIDs {
		ip, err := vagrant.PublicIP(instanceID)
		if err != nil {
			log.WithError(err).Infof("Failed to retrieve IP address for %s.", instanceID)
		}
		instance := Machine{
			ID:        instanceID,
			PublicIP:  ip,
			PrivateIP: ip,
			Provider:  db.Vagrant,
			Size:      vagrant.Size(instanceID),
		}
		machines = append(machines, instance)
	}
	return machines, nil
}

func (clst vagrantCluster) Stop(machines []Machine) error {
	vagrant := clst.vagrant
	if machines == nil {
		return nil
	}
	for _, m := range machines {
		err := vagrant.Destroy(m.ID)
		if err != nil {
			return err
		}
	}
	return nil
}

func (clst vagrantCluster) Disconnect() {

}

func (clst vagrantCluster) PickBestSize(ram dsl.Range, cpu dsl.Range, maxPrice float64) string {
	return clst.vagrant.CreateSize(ram.Min, cpu.Min)
}
