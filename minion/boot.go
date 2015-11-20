package main

import (
	"fmt"
	"os"

	"github.com/fsouza/go-dockerclient"
)

const ETCD = "quay.io/coreos/etcd:v2.1.3"
const OVN_NORTHD = "melvinw/ovn-northd"
const OVN_CONTROLLER = "melvinw/ovn-controller"
const OVS_VSWITCHD = "melvinw/ovs-vswitchd"
const OVS_OVSDB = "melvinw/ovsdb"
const DOCKER_SOCK_PATH = "unix:///var/run/docker.sock"

func pullSingleImage(client *docker.Client, image string) error {
	return client.PullImage(docker.PullImageOptions{
		Repository:   image,
		OutputStream: os.Stdout},
		docker.AuthConfiguration{})
}

/* Pre-pulls the images necessary by the module so that when we get boot
* instructions we can just go. */
func PullImages() {
	images := []string{ETCD, OVN_NORTHD, OVN_CONTROLLER, OVS_VSWITCHD,
		OVS_OVSDB}

	client, err := docker.NewClient(DOCKER_SOCK_PATH)
	if err != nil {
		log.Warning("Failed to create docker client: %s", err)
		return
	}

	for _, image := range images {
		go func() {
			err := pullSingleImage(client, image)
			if err != nil {
				log.Warning("Failed to pull docker image: %s", err)
			}
		}()
	}
}

func runContainer(client *docker.Client, name, image string,
	binds []string, args []string) error {
	log.Info("Attempting to boot %s", name)
	err := pullSingleImage(client, image)
	if err != nil {
		return err
	}

	container, err := client.CreateContainer(docker.CreateContainerOptions{
		Name: name,
		Config: &docker.Config{
			Image: image,
			Cmd:   args,
		}})
	if err != nil {
		return err
	}

	err = client.StartContainer(container.ID,
		&docker.HostConfig{NetworkMode: "host", Binds: binds})
	if err != nil {
		return err
	}

	log.Info("Successfully booted %s", name)
	return nil
}

func BootWorker(etcdToken string) error {
	client, err := docker.NewClient(DOCKER_SOCK_PATH)
	if err != nil {
		return err
	}

	err = runContainer(client, "etcd-client", ETCD,
		[]string{"/usr/share/ca-certificates:/etc/ssl/certs"},
		[]string{"--discovery=" + etcdToken, "--proxy=on"})

	if err != nil {
		return err
	}

	ovs_binds := []string{"/var/run/ovs:/usr/local/var/run/openvswitch:rw"}

	err = runContainer(client, "ovn-controller", OVN_CONTROLLER, ovs_binds, []string{})
	if err != nil {
		return err
	}

	err = runContainer(client, "ovs-vswitchd", OVS_VSWITCHD, ovs_binds, []string{})
	if err != nil {
		return err
	}

	err = runContainer(client, "ovsdb", OVS_OVSDB, ovs_binds, []string{})
	if err != nil {
		return err
	}

	return nil
}

func BootMaster(etcdToken string, ip string) error {
	client, err := docker.NewClient(DOCKER_SOCK_PATH)
	if err != nil {
		return err
	}

	advertiseClient := fmt.Sprintf("http://%s:2379", ip)
	listenClient := fmt.Sprintf("http://0.0.0.0:2379")

	initialAdvertisePeer := fmt.Sprintf("http://%s:2380", ip)
	listenPeer := fmt.Sprintf("http://%s:2380", ip)

	args := []string{"--discovery=" + etcdToken,
		"--advertise-client-urls=" + advertiseClient,
		"--initial-advertise-peer-urls=" + initialAdvertisePeer,
		"--listen-client-urls=" + listenClient,
		"--listen-peer-urls=" + listenPeer}

	binds := []string{"/usr/share/ca-certificates:/etc/ssl/certs"}

	err = runContainer(client, "etcd-master", ETCD, binds, args)
	if err != nil {
		return err
	}

	binds = []string{"/var/run/ovs:/usr/local/var/run/openvswitch:rw"}
	err = runContainer(client, "ovn-northd", OVN_NORTHD, binds, []string{})
	if err != nil {
		return err
	}

	return nil
}
