package supervisor

import (
	"bytes"
	"fmt"
	"net/http"
	"time"

	"github.com/NetSys/di/container"
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

/* Supervisors are responsible for managing containers running on the system.  This
* means booting and tearing down containers when the role of a minion changes.  It also
* means responsonding to ETCD leader elections. */
type Supervisor struct {
	dk  *docker.Client
	cfg MinionConfig

	running       map[string]bool
	containerChan chan ContainerConfig
	done          chan<- struct{}
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

/* Create a new supervisor. Blocks until successful. */
func New(cntrChan chan ContainerConfig) *Supervisor {
	var dk *docker.Client
	for {
		var err error
		dk, err = docker.NewClient(DOCKER_SOCK_PATH)
		if err != nil {
			log.Warning("Failed to create docker client: %s", err)
		} else {
			break
		}
		time.Sleep(10 * time.Second)
	}

	sv := &Supervisor{
		dk:            dk,
		running:       make(map[string]bool),
		containerChan: cntrChan,
	}

	images := []string{OVS_OVSDB, OVN_NORTHD, OVN_CONTROLLER, OVS_VSWITCHD, ETCD}
	for _, image := range images {
		go sv.pullImage(image)
	}

	return sv
}

/* Update the supervisors minion configuration.  If 'cfg' changes, this will cause all
* service containers to be stopped, and new ones rebooted appropriate for the role
* change. */
func (sv *Supervisor) Configure(cfg MinionConfig) {
	if sv.cfg == cfg {
		return
	}
	sv.cfg = cfg

	sv.stopEverything()

	done := make(chan struct{})
	sv.done = done

	switch cfg.Role {
	case MinionConfig_MASTER:
		err := sv.startMaster(done)
		if err != nil {
			log.Warning("Failed to stat master containers: %s", err)
			sv.stopEverything()
		}
		return
	case MinionConfig_WORKER:
		err := sv.startWorker(done)
		if err != nil {
			log.Warning("Failed to stat worker containers: %s", err)
			sv.stopEverything()
		}
		return
	case MinionConfig_NONE:
		return
	default:
		log.Warning("Unknown Minion Role")
	}
}

func (sv Supervisor) stopEverything() {
	if sv.done != nil {
		close(sv.done)
		sv.done = nil
	}

	for name := range sv.running {
		sv.removeContainer(name)
		delete(sv.running, name)
	}
}

func (sv Supervisor) startMaster(done <-chan struct{}) error {
	advertiseClient := fmt.Sprintf("http://%s:2379", sv.cfg.PrivateIP)
	listenClient := fmt.Sprintf("http://0.0.0.0:2379")

	initialAdvertisePeer := fmt.Sprintf("http://%s:2380", sv.cfg.PrivateIP)
	listenPeer := fmt.Sprintf("http://%s:2380", sv.cfg.PrivateIP)

	etcdArgs := []string{"--name=master-" + sv.cfg.PrivateIP,
		"--discovery=" + sv.cfg.EtcdToken,
		"--advertise-client-urls=" + advertiseClient,
		"--initial-advertise-peer-urls=" + initialAdvertisePeer,
		"--listen-client-urls=" + listenClient,
		"--listen-peer-urls=" + listenPeer}

	if err := sv.runContainer("etcd", ETCD, defaultHC(), etcdArgs); err != nil {
		return err
	}

	if err := sv.runContainer("ovsdb-server", OVS_OVSDB, ovsHC(), nil); err != nil {
		return err
	}

	kubeletArgs := []string{"/usr/bin/boot-master", sv.cfg.PrivateIP}
	err := sv.runContainer("di-kubelet", KUBELET, kubeHC(), kubeletArgs)
	if err != nil {
		return err
	}

	go sv.runMaster(done)
	return nil
}

func (sv Supervisor) startWorker(done <-chan struct{}) error {
	etcdArgs := []string{"--discovery=" + sv.cfg.EtcdToken, "--proxy=on"}
	if err := sv.runContainer("etcd", ETCD, defaultHC(), etcdArgs); err != nil {
		return err
	}

	hc := ovsHC()
	if err := sv.runContainer("ovsdb-server", OVS_OVSDB, hc, nil); err != nil {
		return err
	}

	if err := sv.runContainer("ovn-controller", OVN_CONTROLLER, hc, nil); err != nil {
		return err
	}

	hc.Privileged = true
	if err := sv.runContainer("ovs-vswitchd", OVS_VSWITCHD, hc, nil); err != nil {
		return err
	}

	go sv.runWorker(done)
	return nil
}

func (sv Supervisor) runWorker(done <-chan struct{}) {
	chn := make(chan string)
	go watchLeader(chn, done)

	for {
		var leaderIP string
		select {
		case <-done:
			return
		case leaderIP = <-chn:
		}

		if err := sv.removeContainer("di-kubelet"); err != nil {
			log.Warning("Failed to remove di-kubelet: %s", err)
		}

		if leaderIP == "" {
			log.Info("Operating without a leader")
			continue
		}

		args := []string{"/usr/bin/boot-worker", sv.cfg.PrivateIP, leaderIP}
		err := sv.runContainer("di-kubelet", KUBELET, kubeHC(), args)
		if err != nil {
			log.Warning("Failed to boot di-kublet: %s", err)
		}

		cmd := []string{
			"ovs-vsctl",
			"set",
			"Open_vSwitch",
			".",
			fmt.Sprintf("external_ids:ovn-remote=\"tcp:%s:6640\"", leaderIP),
			fmt.Sprintf("external_ids:ovn-encap-ip=%s", sv.cfg.PrivateIP),
			"external_ids:ovn-encap-type=\"geneve\""}
		if err := sv.execInContainer("ovs-vswitchd", cmd); err != nil {
			log.Warning("Failed to reconfigure ovn: %s", err)
		}
	}
}

func (sv Supervisor) runMaster(done <-chan struct{}) {
	/* Create the kube-system namespace. This is where info about all of the
	 * kubernetes pods, conatiners, replication controllers, etc... will
	 * live.
	 *
	 * XXX: Should we create a separate namespace for user pods, e.g.
	 * the AWS namespace we use right now? There is probably a convention
	 * for this mentioned somewhere in the kubernetes community. */
	body := `{"apiVersion":"v1","kind":"Namespace",` +
		`"metadata":{"name":"kube-system"}}`
	url := "http://127.0.0.1:9000/api/v1/namespaces"
	ctype := "application/json"
OuterLoop:
	for {
		select {
		case <-done:
			return
		default:
			_, err := http.Post(url, ctype, bytes.NewBuffer([]byte(body)))
			if err == nil {
				break OuterLoop
			}
		}
		time.Sleep(5 * time.Second)
	}

	chn := make(chan bool)
	cntrChan := make(chan map[string]int32)
	go campaign(sv.cfg.PrivateIP, chn, done)
	go container.Run(container.KUBERNETES, cntrChan)
	var leader bool
	for {
		var cntrCfg ContainerConfig
		select {
		case leader = <-chn:
		case cntrCfg = <-sv.containerChan:
			if leader {
				cntrChan <- cntrCfg.Count
			}
			continue
		case <-done:
			return
		}

		if leader {
			err := sv.runContainer("ovn-northd", OVN_NORTHD, ovsHC(), nil)
			if err != nil {
				/* XXX: If we fail to boot ovn-northd, we should give up
				 * our leadership somehow.  This ties into the general
				 * problem of monitoring health. */
				log.Warning("Failed to boot ovn-northd: %s", err)
			}
		} else if err := sv.removeContainer("ovn-northd"); err != nil {
			log.Warning("Failed to remove ovn-northd: %s", err)
		}
	}
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
		} else {
			return err
		}
	} else {
		log.Info("Successfully booted %s", name)
	}

	sv.running[name] = true
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
		return nil
	}

	err := sv.dk.RemoveContainer(docker.RemoveContainerOptions{ID: *id, Force: true})
	if err != nil {
		return err
	}

	return nil
}
