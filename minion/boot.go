package main

import (
	"fmt"
	"os"

	"github.com/fsouza/go-dockerclient"
	"github.com/op/go-logging"
)

var log = logging.MustGetLogger("main")
var ETCD = "quay.io/coreos/etcd:v2.1.3"
var DOCKER_SOCK_PATH = "unix:///var/run/docker.sock"

func runContainer(client *docker.Client, name, image string,
	args []string) error {
	err := client.PullImage(docker.PullImageOptions{
		Repository:   image,
		OutputStream: os.Stdout},
		docker.AuthConfiguration{})
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
		&docker.HostConfig{NetworkMode: "host"})
	if err != nil {
		return err
	}

	return nil
}

func etcdArgs(etcdToken string, args ...string) []string {
	return append(args, "--discovery="+etcdToken,
		"-v /etc/ssl/certs:/etc/ssl/certs")
}

func BootWorker(etcdToken string) error {
	client, err := docker.NewClient(DOCKER_SOCK_PATH)
	if err != nil {
		return err
	}

	log.Info("Attempting to boot etcd-client")
	err = runContainer(client, "etcd-client", ETCD,
		etcdArgs(etcdToken, "--proxy=on"))
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

	args := etcdArgs(etcdToken,
		"--advertise-client-urls="+advertiseClient,
		"--initial-advertise-peer-urls="+initialAdvertisePeer,
		"--listen-client-urls="+listenClient,
		"--listen-peer-urls="+listenPeer)

	log.Info("Attempting to boot etcd-master")
	err = runContainer(client, "etcd-master", ETCD, args)
	if err != nil {
		return err
	}

	log.Info("Successfully booted etcd-master")
	return err
}
