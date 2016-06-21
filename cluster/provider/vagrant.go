package provider

import (
	"sync"

	"github.com/NetSys/quilt/db"
	"github.com/NetSys/quilt/stitch"
	log "github.com/Sirupsen/logrus"
	"github.com/satori/go.uuid"
)

type vagrantCluster struct {
	namespace string
	vagrant   vagrantAPI
}

func (clst *vagrantCluster) Connect(namespace string) error {
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

	// If any of the bootMachine() calls fail, errChan will contain exactly one error
	// for this function to return.
	errChan := make(chan error, 1)

	var wg sync.WaitGroup
	for _, m := range bootSet {
		wg.Add(1)
		go func(m Machine) {
			defer wg.Done()
			if err := bootMachine(clst.vagrant, m); err != nil {
				select {
				case errChan <- err:
				default:
				}
			}
		}(m)
	}
	wg.Wait()

	var err error
	select {
	case err = <-errChan:
	default:
	}

	return err
}

func bootMachine(vagrant vagrantAPI, m Machine) error {
	id := uuid.NewV4().String()

	err := vagrant.Init(cloudConfigUbuntu(m.SSHKeys, "vivid"), m.Size, id)
	if err == nil {
		err = vagrant.Up(id)
	}

	if err != nil {
		vagrant.Destroy(id)
	}

	return err
}

func (clst vagrantCluster) List() ([]Machine, error) {
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

func (clst vagrantCluster) SetACLs(acls []string) error {
	return nil
}

func (clst vagrantCluster) ChooseSize(ram stitch.Range, cpu stitch.Range,
	maxPrice float64) string {
	return clst.vagrant.CreateSize(ram.Min, cpu.Min)
}
