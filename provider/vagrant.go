package provider

import (
	"os"
	"sync"

	"github.com/NetSys/di/db"
	"github.com/satori/go.uuid"
)

const boxName string = "coreos-beta"
const boxLink string = "http://beta.release.core-os.net/amd64-usr/current/coreos_production_vagrant.json"

type vagrantCluster struct {
	cloudConfig string
	namespace   string
	cwd         string
	vagrant     vagrantAPI
}

func (clst *vagrantCluster) Start(conn db.Conn, clusterID int, namespace string, keys []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	vagrant := newVagrantAPI(cwd)
	err = vagrant.AddBox(boxName, boxLink)
	if err != nil {
		return err
	}
	clst.namespace = namespace
	clst.cloudConfig = cloudConfigCoreOS(append(keys, vagrantPublicKey))
	clst.cwd = cwd
	clst.vagrant = vagrant
	return nil
}

func (clst vagrantCluster) Boot(count int) error {
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
		if err == nil {
			instance := Machine{
				ID:        instanceID,
				PublicIP:  ip,
				PrivateIP: ip,
			}
			machines = append(machines, instance)
		} else {
			/* Boot blocks, so if the VM isn't up, something is wrong. */
			vagrant.Destroy(instanceID)
		}
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
