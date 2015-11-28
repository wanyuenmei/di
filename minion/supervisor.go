package main

import (
	"fmt"

	. "github.com/NetSys/di/minion/proto"
	"github.com/fsouza/go-dockerclient"
)

const ETCD = "quay.io/coreos/etcd:v2.1.3"
const OVN_NORTHD = "quay.io/netsys/ovn-northd"
const OVN_CONTROLLER = "quay.io/netsys/ovn-controller"
const OVS_VSWITCHD = "quay.io/netsys/ovs-vswitchd"
const OVS_OVSDB = "quay.io/netsys/ovsdb-server"
const DOCKER_SOCK_PATH = "unix:///var/run/docker.sock"

type Supervisor struct {
	dk *docker.Client
	cfg MinionConfig
}

func defaultHC() *docker.HostConfig {
	return &docker.HostConfig{
		NetworkMode: "host",
		Binds:       []string{"/usr/share/ca-certificates:/etc/ssl/certs"}}
}

func ovsHC() *docker.HostConfig {
	hc := defaultHC()
	hc.VolumesFrom = []string{"ovsdb-server"}
	return hc
}

func (sv Supervisor) pullImage(image string) error {
	return sv.dk.PullImage(docker.PullImageOptions{
		Repository: image},
		docker.AuthConfiguration{})
}

func NewSupervisor() (*Supervisor, error) {
	dk, err := docker.NewClient(DOCKER_SOCK_PATH)
	if err != nil {
		log.Warning("Failed to create docker client: %s", err)
		return nil, err
	}

	sv := &Supervisor{dk: dk}

	images := []string{OVS_OVSDB, OVN_NORTHD, OVN_CONTROLLER, OVS_VSWITCHD, ETCD}
	for _, image := range images {
		go sv.pullImage(image)
	}

	return sv, nil
}

func (sv Supervisor) runContainer(name, image string, hc *docker.HostConfig,
	args []string) error {

	if err := sv.pullImage(image); err != nil {
		return err
	}

	container, err := sv.dk.CreateContainer(docker.CreateContainerOptions{
		Name: name,
		Config: &docker.Config{
			Image: image,
			Cmd:   args,
		}})
	if err != nil {
		return err
	}

	if err = sv.dk.StartContainer(container.ID, hc); err != nil {
		return err
	}

	log.Info("Successfully booted %s", name)
	return nil
}

func (sv *Supervisor) Configure(cfg MinionConfig) error {
	sv.cfg = cfg

	if err := sv.runContainer("ovsdb-server", OVS_OVSDB, ovsHC(), nil); err != nil {
		return err
	}

	var etcdArgs []string
	switch cfg.Role {
	case MinionConfig_MASTER:
		advertiseClient := fmt.Sprintf("http://%s:2379", cfg.PrivateIP)
		listenClient := fmt.Sprintf("http://0.0.0.0:2379")

		initialAdvertisePeer := fmt.Sprintf("http://%s:2380", cfg.PrivateIP)
		listenPeer := fmt.Sprintf("http://%s:2380", cfg.PrivateIP)

		etcdArgs = []string{"--discovery=" + cfg.EtcdToken,
			"--advertise-client-urls=" + advertiseClient,
			"--initial-advertise-peer-urls=" + initialAdvertisePeer,
			"--listen-client-urls=" + listenClient,
			"--listen-peer-urls=" + listenPeer}

		err := sv.runContainer("ovn-northd", OVN_NORTHD, ovsHC(), nil)
		if err != nil {
			return err
		}

	case MinionConfig_WORKER:
		etcdArgs = []string{"--discovery=" + cfg.EtcdToken, "--proxy=on"}

		hc := ovsHC()
		hc.Privileged = true
		err := sv.runContainer("ovs-vswitchd", OVS_VSWITCHD, hc, nil)
		if err != nil {
			return err
		}

		err = sv.runContainer("ovn-controller", OVN_CONTROLLER, ovsHC(), nil)
		if err != nil {
			return err
		}

	default:
		panic(fmt.Sprintf("Unknown minion role: %d", cfg.Role))
	}

	if err := sv.runContainer("etcd", ETCD, defaultHC(), etcdArgs); err != nil {
		return err
	}

	return nil
}
