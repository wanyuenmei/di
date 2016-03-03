package provider

import (
	"sync"

	"github.com/NetSys/di/db"
	"github.com/NetSys/di/dsl"
	log "github.com/Sirupsen/logrus"
	"github.com/satori/go.uuid"
)

type vagrantCluster struct {
	cloudConfig string
	namespace   string
	vagrant     vagrantAPI
}

func (clst *vagrantCluster) Start(conn db.Conn, clusterID int, namespace string, keys []string) error {
	vagrant := newVagrantAPI()
	err := vagrant.AddBox("boxcutter/ubuntu1504", "virtualbox")
	if err != nil {
		return err
	}
	clst.namespace = namespace
	clst.cloudConfig = cloudConfigUbuntu(append(keys, vagrantPublicKey), "vagrant", "vivid")
	clst.vagrant = vagrant
	return nil
}

func (clst vagrantCluster) Boot(bootSet []Machine) error {
	count := len(bootSet)
	vagrant := clst.vagrant
	var wg sync.WaitGroup
	wg.Add(count)
	for booted := 0; booted < count; booted++ {
		id := uuid.NewV4().String()
		err := vagrant.Init(clst.cloudConfig, id)
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
		}
		machines = append(machines, instance)
	}
	return machines, nil
}

func (clst vagrantCluster) Stop(ids []string) error {
	vagrant := clst.vagrant
	if ids == nil {
		return nil
	}
	for _, id := range ids {
		err := vagrant.Destroy(id)
		if err != nil {
			return err
		}
	}
	return nil
}

func (clst vagrantCluster) Disconnect() {

}

func (clst vagrantCluster) PickBestSize(ram dsl.Range, cpu dsl.Range, maxPrice float64) string {
	return ""
}
