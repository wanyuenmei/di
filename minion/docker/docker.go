package docker

import (
	"archive/tar"
	"bufio"
	"bytes"
	"errors"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/NetSys/quilt/util"

	log "github.com/Sirupsen/logrus"
	dkc "github.com/fsouza/go-dockerclient"
)

const (
	// The root namespace for all labels
	labelBase = "q."

	// This is the namespace for user defined labels
	userLabelPrefix = labelBase + "user.label."

	// This is the namespace for system defined labels
	systemLabelPrefix = labelBase + "system.label."

	// This is the namespace for labels used for placement based on ports.
	portLabelPrefix = systemLabelPrefix + "port."

	// LabelTrueValue is needed because a label has to be a key/value pair, hence
	// this is the value that will be used if we're only interested in the key
	LabelTrueValue = "1"

	// SchedulerLabelKey is the key, used by the scheduler.
	SchedulerLabelKey = systemLabelPrefix + "Quilt"

	// SchedulerLabelValue is the value, used by the scheduler.
	SchedulerLabelValue = "Scheduler"

	// SchedulerLabelPair is the key/value pair, used by the scheduler.
	SchedulerLabelPair = SchedulerLabelKey + "=" + SchedulerLabelValue
)

var errNoSuchContainer = errors.New("container does not exist")

// A Container as returned by the docker client API.
type Container struct {
	ID     string
	Name   string
	Image  string
	IP     string
	Path   string
	Args   []string
	Pid    int
	Env    map[string]string
	Labels map[string]string
}

// ContainerSlice is an alias for []Container to allow for joins
type ContainerSlice []Container

// A Client to the local docker daemon.
type Client interface {
	Run(opts RunOptions) error
	Exec(name string, cmd ...string) error
	ExecVerbose(name string, cmd ...string) ([]byte, []byte, error)
	Remove(name string) error
	RemoveID(id string) error
	Pull(image string) error
	List(filters map[string][]string) ([]Container, error)
	Get(id string) (Container, error)
	WriteToContainer(id, src, dst, archiveName string, permission int) error
	GetFromContainer(id string, src string) (string, error)
	IsRunning(name string) (bool, error)
}

// RunOptions changes the behavior of the Run function.
type RunOptions struct {
	Name   string
	Image  string
	Args   []string
	Labels map[string]string
	Env    map[string]struct{}

	Binds       []string
	NetworkMode string
	PidMode     string
	Privileged  bool
	VolumesFrom []string
}

type pullRequest struct {
	image string
	done  chan error
}

type docker struct {
	*dkc.Client

	pullChan chan pullRequest
}

// New creates client to the docker daemon.
func New(sock string) Client {
	var client *dkc.Client
	for {
		var err error
		client, err = dkc.NewClient(sock)
		if err != nil {
			log.WithError(err).Warn("Failed to create docker client.")
			time.Sleep(10 * time.Second)
			continue
		}
		break
	}

	dk := docker{client, make(chan pullRequest)}
	go pullServer(dk)

	return dk
}

func pullServer(dk docker) {
	images := make(map[string]struct{})

	for req := range dk.pullChan {
		if _, ok := images[req.image]; ok {
			req.done <- nil
			continue
		}

		log.Infof("Pulling docker image %s.", req.image)
		opts := dkc.PullImageOptions{Repository: string(req.image)}
		err := dk.PullImage(opts, dkc.AuthConfiguration{})

		if err != nil {
			log.WithError(err).Errorf("Failed to pull image %s.", req.image)
		} else {
			images[req.image] = struct{}{}
		}
		req.done <- err
	}
}

func (dk docker) Run(opts RunOptions) error {
	if opts.Name != "" {
		_, err := dk.getID(opts.Name)
		if err == errNoSuchContainer {
			// Only log the first time we attempt to boot.
			log.Infof("Start Container: %s", opts.Name)
		} else if err != nil {
			return err
		}
	}

	id, err := dk.create(opts.Name, opts.Image, opts.Args, opts.Labels, opts.Env)
	if err != nil {
		return err
	}

	hc := dkc.HostConfig{
		Binds:       opts.Binds,
		NetworkMode: opts.NetworkMode,
		PidMode:     opts.PidMode,
		Privileged:  opts.Privileged,
		VolumesFrom: opts.VolumesFrom,
	}
	if err = dk.StartContainer(id, &hc); err != nil {
		if _, ok := err.(*dkc.ContainerAlreadyRunning); ok {
			return nil
		}
		dk.removeID(id) // Remove the container to avoid a zombie.
		return err
	}

	return nil
}

func (dk docker) Exec(name string, cmd ...string) error {
	_, _, err := dk.ExecVerbose(name, cmd...)
	return err
}

func (dk docker) ExecVerbose(name string, cmd ...string) ([]byte, []byte, error) {
	id, err := dk.getID(name)
	if err != nil {
		return nil, nil, err
	}

	var inBuff, outBuff bytes.Buffer
	exec, err := dk.CreateExec(dkc.CreateExecOptions{
		Container:    id,
		Cmd:          cmd,
		AttachStdout: true})

	if err != nil {
		return nil, nil, err
	}

	err = dk.StartExec(exec.ID, dkc.StartExecOptions{
		OutputStream: &inBuff,
	})

	if err != nil {
		return nil, nil, err
	}

	scanner := bufio.NewScanner(bytes.NewReader(inBuff.Bytes()))
	for scanner.Scan() {
		outBuff.WriteString(scanner.Text() + "\n")
	}
	if err := scanner.Err(); err != nil {
		return nil, nil, err
	}

	return outBuff.Bytes(), outBuff.Bytes(), nil
}

// WriteToContainer writes the contents of SRC into the file at path DST on the
// container with id ID. Overwrites DST if it already exists.
func (dk docker) WriteToContainer(id, src, dst, archiveName string,
	permission int) error {

	tarBuf, err := util.ToTar(archiveName, permission, src)

	if err != nil {
		return err
	}

	err = dk.UploadToContainer(id, dkc.UploadToContainerOptions{
		InputStream: tarBuf,
		Path:        dst,
	})
	if err != nil {
		return err
	}

	return nil
}

// GetFromContainer returns a string containing the content of the file named
// SRC on the container with id ID.
func (dk docker) GetFromContainer(id string, src string) (string, error) {
	var buffIn bytes.Buffer
	var buffOut bytes.Buffer
	err := dk.DownloadFromContainer(id, dkc.DownloadFromContainerOptions{
		OutputStream: &buffIn,
		Path:         src,
	})
	if err != nil {
		return "", err
	}

	writer := io.Writer(&buffOut)

	for tr := tar.NewReader(&buffIn); err != io.EOF; _, err = tr.Next() {

		if err != nil {
			return "", err
		}

		_, err = io.Copy(writer, tr)
		if err != nil {
			return "", err
		}
	}

	return buffOut.String(), nil
}

func (dk docker) Remove(name string) error {
	id, err := dk.getID(name)
	if err != nil {
		return nil // Can't remove a non-existent container.
	}

	log.WithFields(log.Fields{
		"name": name,
		"id":   id,
	}).Info("Remove container.")
	return dk.removeID(id)
}

func (dk docker) RemoveID(id string) error {
	log.WithField("id", id).Info("Remove Container.")
	return dk.removeID(id)
}

func (dk docker) removeID(id string) error {
	err := dk.RemoveContainer(dkc.RemoveContainerOptions{ID: id, Force: true})
	if err != nil {
		return err
	}

	return nil
}

func (dk docker) Pull(image string) error {
	done := make(chan error)
	dk.pullChan <- pullRequest{image, done}
	return <-done
}

func (dk docker) List(filters map[string][]string) ([]Container, error) {
	return dk.list(filters, false)
}

func (dk docker) list(filters map[string][]string, all bool) ([]Container, error) {
	opts := dkc.ListContainersOptions{All: all, Filters: filters}
	apics, err := dk.ListContainers(opts)
	if err != nil {
		return nil, err
	}

	var containers []Container
	for _, apic := range apics {
		c, err := dk.Get(apic.ID)
		if err != nil {
			log.WithError(err).Warnf("Failed to inspect container: %s",
				apic.ID)
			continue
		}

		containers = append(containers, c)
	}

	return containers, nil
}

func (dk docker) Get(id string) (Container, error) {
	c, err := dk.InspectContainer(id)
	if err != nil {
		return Container{}, err
	}

	env := make(map[string]string)
	for _, value := range c.Config.Env {
		e := strings.Split(value, "=")
		if len(e) > 1 {
			env[e[0]] = e[1]
		}
	}

	return Container{
		Name:   c.Name,
		ID:     c.ID,
		IP:     c.NetworkSettings.IPAddress,
		Image:  c.Config.Image,
		Path:   c.Path,
		Args:   c.Args,
		Pid:    c.State.Pid,
		Env:    env,
		Labels: c.Config.Labels,
	}, nil
}

func (dk docker) IsRunning(name string) (bool, error) {
	containers, err := dk.List(map[string][]string{
		"name": {name},
	})
	if err != nil {
		return false, err
	}
	return len(containers) != 0, nil
}

func (dk docker) create(name, image string, args []string,
	labels map[string]string, env map[string]struct{}) (string, error) {
	if err := dk.Pull(image); err != nil {
		return "", err
	}

	envList := make([]string, len(env))
	for k := range env {
		envList = append(envList, k)
	}

	container, err := dk.CreateContainer(dkc.CreateContainerOptions{
		Name:   name,
		Config: &dkc.Config{Image: string(image), Cmd: args, Labels: labels, Env: envList},
	})
	if err != nil {
		return "", err
	}

	return container.ID, nil
}

func (dk docker) getID(name string) (string, error) {
	containers, err := dk.list(nil, true)
	if err != nil {
		return "", err
	}

	name = "/" + name
	for _, c := range containers {
		if name == c.Name {
			return c.ID, nil
		}
	}

	return "", errNoSuchContainer
}

// UserLabel returns the supplied label tagged with the user prefix.
func UserLabel(label string) string {
	return userLabelPrefix + label
}

// IsUserLabel returns whether the supplied label represents a Quilt user label.
func IsUserLabel(label string) bool {
	return strings.HasPrefix(label, userLabelPrefix)
}

// ParseUserLabel returns the supplied label with the user prefix stripped.
func ParseUserLabel(label string) string {
	return strings.TrimPrefix(label, userLabelPrefix)
}

// SystemLabel returns the supplied label tagged with the system prefix.
func SystemLabel(label string) string {
	return systemLabelPrefix + label
}

// PortLabel returns the supplied label tagged with the port prefix.
func PortLabel(port int) string {
	return portLabelPrefix + strconv.Itoa(port)
}

// Get returns the value contained at the given index
func (cs ContainerSlice) Get(ii int) interface{} {
	return cs[ii]
}

// Len returns the number of items in the slice
func (cs ContainerSlice) Len() int {
	return len(cs)
}
