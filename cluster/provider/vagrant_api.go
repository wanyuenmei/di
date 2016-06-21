package provider

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	"github.com/NetSys/quilt/util"
	homedir "github.com/mitchellh/go-homedir"
)

var vagrantCmd = "vagrant"
var shCmd = "sh"

type vagrantAPI struct{}

func newVagrantAPI() vagrantAPI {
	vagrant := vagrantAPI{}
	return vagrant
}

func (api vagrantAPI) Init(cloudConfig string, size string, id string) error {
	vdir, err := api.VagrantDir()
	if err != nil {
		return err
	}
	if _, err := os.Stat(vdir); os.IsNotExist(err) {
		os.Mkdir(vdir, os.ModeDir|os.ModePerm)
	}
	path := vdir + id
	os.Mkdir(path, os.ModeDir|os.ModePerm)

	_, err = api.Shell(id, `vagrant --machine-readable init coreos-beta`)
	if err != nil {
		api.Destroy(id)
		return err
	}

	err = util.WriteFile(path+"/user-data", []byte(cloudConfig), 0644)
	if err != nil {
		api.Destroy(id)
		return err
	}

	vagrant := vagrantFile()
	err = util.WriteFile(path+"/vagrantFile", []byte(vagrant), 0644)
	if err != nil {
		api.Destroy(id)
		return err
	}

	err = util.WriteFile(path+"/size", []byte(size), 0644)
	if err != nil {
		api.Destroy(id)
		return err
	}

	return nil
}

func (api vagrantAPI) Up(id string) error {
	_, err := api.Shell(id, `vagrant --machine-readable up`)
	if err != nil {
		return err
	}
	return nil
}

func (api vagrantAPI) Destroy(id string) error {
	_, err := api.Shell(id, `vagrant --machine-readable destroy -f; cd ../; rm -rf %s`)
	if err != nil {
		return err
	}
	return nil
}

func (api vagrantAPI) PublicIP(id string) (string, error) {
	ip, err := api.Shell(id, `vagrant ssh -c "ip address show eth1 | grep 'inet ' | " +
		"sed -e 's/^.*inet //' -e 's/\/.*$//' | tr -d '\n'"`)
	if err != nil {
		return "", err
	}
	return string(ip[:]), nil
}

func (api vagrantAPI) Status(id string) (string, error) {
	output, err := api.Shell(id, `vagrant --machine-readable status`)
	if err != nil {
		return "", err
	}
	lines := bytes.Split(output, []byte("\n"))
	for _, line := range lines {
		words := strings.Split(string(line[:]), ",")
		if len(words) >= 4 {
			if strings.Compare(words[2], "state") == 0 {
				return words[3], nil
			}
		}
	}
	return "", nil
}

func (api vagrantAPI) List() ([]string, error) {
	subdirs := []string{}
	vdir, err := api.VagrantDir()
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(vdir); os.IsNotExist(err) {
		return subdirs, nil
	}

	files, err := ioutil.ReadDir(vdir)
	if err != nil {
		return subdirs, err
	}
	for _, file := range files {
		subdirs = append(subdirs, file.Name())
	}
	return subdirs, nil
}

func (api vagrantAPI) AddBox(name string, provider string) error {
	/* Adding a box fails if it already exists, hence the check. */
	exists, err := api.ContainsBox(name)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	err = exec.Command(vagrantCmd, []string{"--machine-readable", "box", "add",
		"--provider", provider, name}...).Run()
	if err != nil {
		return err
	}
	return nil
}

func (api vagrantAPI) ContainsBox(name string) (bool, error) {
	output, err := exec.Command(vagrantCmd, []string{"--machine-readable", "box",
		"list"}...).Output()
	if err != nil {
		return false, err
	}
	lines := bytes.Split(output, []byte("\n"))
	for _, line := range lines {
		words := strings.Split(string(line[:]), ",")
		if words[len(words)-1] == name {
			return true, nil
		}
	}
	return false, nil
}

func (api vagrantAPI) Shell(id string, commands string) ([]byte, error) {
	chdir := `(cd %s; `
	vdir, err := api.VagrantDir()
	if err != nil {
		return nil, err
	}
	chdir = fmt.Sprintf(chdir, vdir+id)
	shellCommand := chdir + strings.Replace(commands, "%s", id, -1) + ")"
	output, err := exec.Command(shCmd, []string{"-c", shellCommand}...).Output()
	return output, err
}

func (api vagrantAPI) VagrantDir() (string, error) {
	dir, err := homedir.Dir()
	if err != nil {
		return "", err
	}
	vagrantDir := dir + "/.vagrant/"
	return vagrantDir, nil
}

func (api vagrantAPI) Size(id string) string {
	size, err := api.Shell(id, "cat size")
	if err != nil {
		return ""
	}
	return string(size)
}

func vagrantFile() string {
	vagrantfile := `CLOUD_CONFIG_PATH = File.join(File.dirname(__FILE__), "user-data")
SIZE_PATH = File.join(File.dirname(__FILE__), "size")
Vagrant.require_version ">= 1.6.0"

size = File.open(SIZE_PATH).read.strip.split(",")
Vagrant.configure(2) do |config|
  config.vm.box = "boxcutter/ubuntu1504"

  config.vm.network "private_network", type: "dhcp"

  ram=(size[0].to_f*1024).to_i
  cpus=size[1]
  config.vm.provider "virtualbox" do |v|
    v.memory = ram
    v.cpus = cpus
  end

  if File.exist?(CLOUD_CONFIG_PATH)
    config.vm.provision "shell", path: "#{CLOUD_CONFIG_PATH}"
  end
end
`
	return vagrantfile
}

// VagrantCreateSize creates an encoded string representing the amount of RAM
// and number of CPUs for an instance.
func (api vagrantAPI) CreateSize(ram, cpu float64) string {
	if ram < 1 {
		ram = 1
	}
	if cpu < 1 {
		cpu = 1
	}
	return fmt.Sprintf("%g,%g", ram, cpu)
}
