package main

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"time"

	. "github.com/NetSys/di/minion/proto"
	"github.com/fsouza/go-dockerclient"
)

const ETCD = "quay.io/coreos/etcd:v2.1.3"
const KUBELET = "quay.io/netsys/kubelet"
const OVN_NORTHD = "quay.io/netsys/ovn-northd"
const OVN_CONTROLLER = "quay.io/netsys/ovn-controller"
const OVS_VSWITCHD = "quay.io/netsys/ovs-vswitchd"
const OVS_OVSDB = "quay.io/netsys/ovsdb-server"
const DOCKER_SOCK_PATH = "unix:///var/run/docker.sock"

type Supervisor struct {
	dk  *docker.Client
	cfg MinionConfig
}

func kubeHC() *docker.HostConfig {
	binds := []string{
		"/var/run:/var/run:rw",
		"/var/lib/docker:/var/lib/docker:rw",
		"/etc/docker:/etc/docker:rw",
		"/dev:/dev:rw",
		"/sys:/sys:rw"}
	return &docker.HostConfig{
		Binds:       binds,
		Privileged:  true,
		NetworkMode: "host",
		/* Use PID=host per the kubernetes documentation */
		PidMode: "host"}
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

func (sv Supervisor) getContainer(name string) *string {
	containers, err := sv.dk.ListContainers(docker.ListContainersOptions{All: true})
	if err != nil {
		log.Warning("Failed to list containers: %s", err)
		return nil
	}

	name = "/" + name
	for _, c := range containers {
		for _, cname := range c.Names {
			if name == cname {
				return &c.ID
			}
		}
	}

	return nil
}

func (sv Supervisor) runContainer(name, image string, hc *docker.HostConfig,
	args []string) error {
	log.Info("Booting Container: %s", name)

	if err := sv.pullImage(image); err != nil {
		return err
	}

	id := sv.getContainer(name)
	if id == nil {
		container, err := sv.dk.CreateContainer(docker.CreateContainerOptions{
			Name: name,
			Config: &docker.Config{
				Image: image,
				Cmd:   args,
			},
		})
		if err != nil {
			return err
		}
		id = &container.ID
	}

	err := sv.dk.StartContainer(*id, hc)
	if err != nil {
		if _, ok := err.(*docker.ContainerAlreadyRunning); ok {
			log.Info("Container already running: %s", name)
			return nil
		} else {
			return err
		}
	}

	log.Info("Successfully booted %s", name)
	return nil
}

func (sv Supervisor) execInContainer(name string, cmd []string) error {
	log.Info("Executing in container: %s", name)

	id := sv.getContainer(name)
	exec, err := sv.dk.CreateExec(docker.CreateExecOptions{
		Container: *id,
		Cmd:       cmd})

	if err != nil {
		return err
	}

	err = sv.dk.StartExec(exec.ID, docker.StartExecOptions{})

	if err != nil {
		return err
	}

	return nil
}

func (sv Supervisor) removeContainer(name string) error {
	log.Info("Remove Container: %s", name)

	id := sv.getContainer(name)
	if id == nil {
		log.Info("Failed to stop missing container: %s", name)
		return nil
	}

	err := sv.dk.RemoveContainer(docker.RemoveContainerOptions{ID: *id, Force: true})
	if err != nil {
		return err
	}

	return nil
}

func (sv *Supervisor) Configure(cfg MinionConfig) error {
	sv.cfg = cfg

	/* XXX: We should really being using an ID that we get from the master. */
	hostname, err := os.Hostname()
	if err != nil {
		return err
	}

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

		etcdArgs = []string{"--name=" + hostname,
			"--discovery=" + cfg.EtcdToken,
			"--advertise-client-urls=" + advertiseClient,
			"--initial-advertise-peer-urls=" + initialAdvertisePeer,
			"--listen-client-urls=" + listenClient,
			"--listen-peer-urls=" + listenPeer}

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

	if cfg.Role == MinionConfig_MASTER {
		kubeletArgs := []string{"/usr/bin/boot-master", sv.cfg.PrivateIP}
		err := sv.runContainer("di-kubelet", KUBELET, kubeHC(), kubeletArgs)
		if err != nil {
			return err
		}

		/* Create the kube-system namespace. This is where info about all of the
		 * kubernetes pods, conatiners, replication controllers, etc... will
		 * live. */
		go func() {
			body := `{"apiVersion":"v1","kind":"Namespace","metadata":{"name":"kube-system"}}`
			url := "http://127.0.0.1:9000/api/v1/namespaces"
			ctype := "application/json"
			for {
				_, err := http.Post(url, ctype, bytes.NewBuffer([]byte(body)))
				if err == nil {
					break
				}
				time.Sleep(5 * time.Second)
			}
		}()

		/* XXX: Should we create a separate namespace for user pods, e.g.
		 * the AWS namespace we use right now? There is probably a convention
		 * for this mentioned somewhere in the kubernetes community. */
	}

	return nil
}

func (sv Supervisor) WatchLeaderChannel(leaderChan chan *string) {
	for {
		leaderIp := <-leaderChan
		if leaderIp == nil {
			log.Warning("Leader went away.")
			/* XXX: Handle this. */
			continue
		}

		kubeletArgs := []string{"/usr/bin/boot-worker", sv.cfg.PrivateIP,
			*leaderIp}
		err := sv.runContainer("di-kubelet", KUBELET, kubeHC(), kubeletArgs)
		if err != nil {
			log.Warning("Failed to boot di-kubelet: %s", err)
			if err := sv.removeContainer("di-kubelet"); err != nil {
				log.Warning("Failed to remove di-kubelet: %s", err)
			}
		}
	}
}

func (sv Supervisor) WatchElectionChannel(electionChan chan bool) {
	for {
		if <-electionChan {
			err := sv.runContainer("ovn-northd", OVN_NORTHD, ovsHC(), nil)
			if err != nil {
				/* XXX: If we fail to boot ovn-northd, we should give up
				 * our leadership somehow.  This ties into the general
				 * problem of monitoring health. */
				log.Warning("Failed to boot ovn-northd: %s", err)
			}
		} else {
			err := sv.removeContainer("ovn-northd")
			if err != nil {
				log.Warning("Failed to remove ovn-northd: %s", err)
			}
		}
	}
}
