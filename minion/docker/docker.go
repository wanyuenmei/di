package docker

import (
	"errors"
	"strings"
	"time"

	dkc "github.com/fsouza/go-dockerclient"
	"github.com/op/go-logging"
)

// An Image is a Docker container which this library supports running.
type Image string

const (
	// Etcd consnesus provider.
	Etcd Image = "quay.io/coreos/etcd:v2.1.3"

	// Kubelet implements the kubernetes cluster scheduler.
	Kubelet Image = "quay.io/netsys/kubelet"

	// Ovnnorthd is the central controller for OVN network virtualization.
	Ovnnorthd Image = "quay.io/netsys/ovn-northd"

	// Ovncontroller is the per host controller for OVN network virtualization.
	Ovncontroller Image = "quay.io/netsys/ovn-controller"

	// Ovnoverlay is the ovn networking driver for docker
	Ovnoverlay Image = "quay.io/netsys/ovn-overlay"

	// Ovsvswitchd is the Open vSwitch control plane.
	Ovsvswitchd Image = "quay.io/netsys/ovs-vswitchd"

	// Ovsdb is teh database for Open vSwitch and OVN.
	Ovsdb Image = "quay.io/netsys/ovsdb-server"
)

const dockerSock = "unix:///var/run/docker.sock"

var log = logging.MustGetLogger("docker")

var errNoSuchContainer = errors.New("container does not exist")

// A Client to the local docker daemon.
type Client interface {
	Run(image Image, args []string) error
	Exec(image Image, cmd []string) error
	Remove(image Image) error
	RemoveAll()
	CreateLSwitch(name string) error
}

type docker struct {
	*dkc.Client
	running map[Image]struct{}
}

// New creates client to the docker daemon.
func New() Client {
	var client *dkc.Client
	for {
		var err error
		client, err = dkc.NewClient(dockerSock)
		if err != nil {
			log.Warning("Failed to create docker client: %s", err)
			time.Sleep(10 * time.Second)
			continue
		}
		break
	}

	dk := docker{client, make(map[Image]struct{})}
	images := []Image{Etcd, Kubelet, Ovnnorthd, Ovncontroller, Ovsvswitchd, Ovsdb}
	for _, image := range images {
		go dk.pull(image)
	}

	return dk
}

func (dk docker) Run(image Image, args []string) error {
	name := image.String()
	_, err := dk.get(name)
	if err == errNoSuchContainer {
		// Only log the first time we attempt to boot.
		log.Info("Start Container: %s", name)
	} else if err != nil {
		log.Warning("Failed to start container: %s", err.Error())
	}

	id, err := dk.create(name, image, args)
	if err != nil {
		log.Warning("Failed to create container %s: %s", name, err.Error())
		return err
	}

	if err = dk.StartContainer(id, hc(image)); err != nil {
		if _, ok := err.(*dkc.ContainerAlreadyRunning); !ok {
			log.Warning("Failed to start container %s: %s", name, err)
		}
		return err
	}

	dk.running[image] = struct{}{}
	return nil
}

func (dk docker) Exec(image Image, cmd []string) error {
	name := image.String()

	id, err := dk.get(name)
	if err != nil {
		return err
	}

	log.Info("Exec in %s: %s", name, strings.Join(cmd, " "))
	exec, err := dk.CreateExec(dkc.CreateExecOptions{Container: id, Cmd: cmd})
	if err != nil {
		log.Warning("Failed to exec %s %s: %s", name, cmd, err.Error())
		return err
	}

	err = dk.StartExec(exec.ID, dkc.StartExecOptions{})
	if err != nil {
		log.Warning("Failed to exec %s %s: %s", name, cmd, err.Error())
		return err
	}

	return nil
}

func (dk docker) Remove(image Image) error {
	name := image.String()

	id, err := dk.get(name)
	if err != nil {
		return err
	}

	log.Info("Remove Container: %s", name)
	err = dk.RemoveContainer(dkc.RemoveContainerOptions{ID: id, Force: true})
	if err != nil {
		log.Warning("Failed to remove container %s: %s", name, err)
		return err
	}

	return nil
}

func (dk docker) RemoveAll() {
	for name := range dk.running {
		dk.Remove(name)
		delete(dk.running, name)
	}
}

func (dk docker) CreateLSwitch(name string) error {
	_, err := dk.CreateNetwork(dkc.CreateNetworkOptions{
		Name:           name,
		CheckDuplicate: true,
		Driver:         "openvswitch"})
	if err != nil {
		log.Warning("Failed to create di lswitch: %s", err)
	}
	return err
}

func (dk docker) create(name string, image Image, args []string) (string, error) {
	dk.pull(image)

	id, err := dk.get(name)
	if err == nil {
		return id, nil
	}

	container, err := dk.CreateContainer(dkc.CreateContainerOptions{
		Name:   name,
		Config: &dkc.Config{Image: string(image), Cmd: args},
	})
	if err != nil {
		log.Warning("Failed to create container: %s", err)
		return "", err
	}

	return container.ID, nil
}

func (dk docker) get(name string) (string, error) {
	containers, err := dk.ListContainers(dkc.ListContainersOptions{All: true})
	if err != nil {
		return "", err
	}

	name = "/" + name
	for _, c := range containers {
		for _, cname := range c.Names {
			if name == cname {
				return c.ID, nil
			}
		}
	}

	return "", errNoSuchContainer
}

func (dk docker) pull(image Image) {
	err := dk.PullImage(dkc.PullImageOptions{Repository: string(image)},
		dkc.AuthConfiguration{})
	if err != nil {
		log.Warning("Failed to pull image: %s", err)
	}
}

func hc(image Image) *dkc.HostConfig {
	hc := dkc.HostConfig{
		NetworkMode: "host",
		Binds:       []string{"/usr/share/ca-certificates:/etc/ssl/certs"}}

	switch image {
	case Ovnoverlay:
		hc.Binds = []string{"/etc/docker:/etc/docker:rw"}
		fallthrough
	case Ovsvswitchd:
		hc.Privileged = true
		fallthrough
	case Ovnnorthd:
		fallthrough
	case Ovncontroller:
		hc.VolumesFrom = []string{Ovsdb.String()}
	case Kubelet:
		hc.Binds = []string{
			"/proc:/hostproc:ro",
			"/var/run:/var/run:rw",
			"/var/lib/docker:/var/lib/docker:rw",
			"/etc/docker:/etc/docker:rw",
			"/dev:/dev:rw",
			"/sys:/sys:ro",
		}
		hc.Privileged = true
		hc.PidMode = "host" // Use PID=host per the kubernetes documentation.
	default:
	}

	return &hc
}

func (image Image) String() string {
	switch image {
	case Etcd:
		return "etcd"
	case Kubelet:
		return "kubelet"
	case Ovnnorthd:
		return "ovn-northd"
	case Ovncontroller:
		return "ovn-controller"
	case Ovnoverlay:
		return "ovn-overlay"
	case Ovsvswitchd:
		return "ovs-vswitchd"
	case Ovsdb:
		return "ovsdb-server"
	default:
		panic("Unimplemented")
	}
}
