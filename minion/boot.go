package main

import (
	"fmt"
	"os"

	"github.com/fsouza/go-dockerclient"
)

const ETCD = "quay.io/coreos/etcd:v2.1.3"
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
	images := []string{ETCD}

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

	return nil
}

func BootWorker(etcdToken string) error {
	client, err := docker.NewClient(DOCKER_SOCK_PATH)
	if err != nil {
		return err
	}

	log.Info("Attempting to boot etcd-client")
	err = runContainer(client, "etcd-client", ETCD,
		[]string{"/usr/share/ca-certificates:/etc/ssl/certs"},
		[]string{"--discovery=" + etcdToken, "--proxy=on"})
	if err != nil {
		return err
	}

	log.Info("Successfully booted etcd-client")
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

	log.Info("Attempting to boot etcd-master")
	err = runContainer(client, "etcd-master", ETCD, binds, args)
	if err != nil {
		return err
	}

	log.Info("Successfully booted etcd-master")
	return err
}
