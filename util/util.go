package util

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
)

const (
	minionImage = "quay.io/netsys/di-minion:latest"
)

func httpRequest(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "<error>", err
	}

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "<error>", err
	}

	return strings.TrimSpace(string(body)), err
}

func MyIp() (string, error) {
	return httpRequest("http://checkip.amazonaws.com/")
}

func CloudConfigUbuntu(keys []string) string {
	cloudConfig := `#!/bin/bash

initialize_ovs() {
	cat <<- EOF > /etc/systemd/system/ovs.service
	[Unit]
	Description=OVS

	[Service]
	ExecStart=/sbin/modprobe openvswitch
	ExecStartPost=/sbin/modprobe vport_geneve

	[Install]
	WantedBy=multi-user.target
	EOF
}

initialize_docker() {
	PRIVATE_IPv4="$(curl http://instance-data/latest/meta-data/local-ipv4)"
	mkdir -p /etc/systemd/system/docker.service.d

	# For networking in docker
	mkdir -p /var/run/netns

	cat <<- EOF > /etc/systemd/system/docker.service.d/override.conf
	[Unit]
	Description=docker

	[Service]
	ExecStart=
	ExecStart=/usr/bin/docker daemon --bridge=none \
	-H "${PRIVATE_IPv4}:2375" -H unix:///var/run/docker.sock \

	[Install]
	WantedBy=multi-user.target
	EOF
}

initialize_minion() {
	cat <<- EOF > /etc/systemd/system/minion.service
	[Unit]
	Description=DI Minion
	After=docker.service
	Requires=docker.service

	[Service]
	TimeoutSec=1000
	ExecStartPre=-/usr/bin/docker kill minion
	ExecStartPre=-/usr/bin/docker rm minion
	ExecStartPre=/usr/bin/docker pull %s
	ExecStart=/usr/bin/docker run --net=host --name=minion --privileged \
	-v /var/run/docker.sock:/var/run/docker.sock %s

	[Install]
	WantedBy=multi-user.target
	EOF
}

install_docker() {
	# Disable default sources list since we don't use them anyways
	mv /etc/apt/sources.list /etc/apt/sources.list.bak

	apt-key adv --keyserver hkp://p80.pool.sks-keyservers.net:80 --recv-keys 58118E89F3A912897C070ADBF76221572C52609D
	echo "deb https://apt.dockerproject.org/repo ubuntu-wily main" > /etc/apt/sources.list.d/docker.list
	apt-get update
	apt-get install docker-engine=1.9.1-0~wily -y
	systemctl stop docker.service
}

echo -n "Start Boot Script: " >> /var/log/bootscript.log
date >> /var/log/bootscript.log

USER_DIR=/home/ubuntu
export DEBIAN_FRONTEND=noninteractive

install_docker
initialize_ovs
initialize_docker
initialize_minion

# Reload because we replaced the docker.service provided by the package
systemctl daemon-reload

# Enable our services to run on boot
systemctl enable {ovs,docker,minion}.service

# Start our services
systemctl restart {ovs,docker,minion}.service

# Create dirs and files with correct users and permissions
install -d -o ubuntu -m 700 $USER_DIR/.ssh
install -o ubuntu -m 600 /dev/null $USER_DIR/.ssh/authorized_keys

# allow the ubuntu user to use docker without sudo
usermod -aG docker ubuntu

echo -n "Completed Boot Script: " >> /var/log/bootscript.log
date >> /var/log/bootscript.log
    `
	cloudConfig = fmt.Sprintf(cloudConfig, minionImage, minionImage)

	if len(keys) > 0 {
		for _, key := range keys {
			cloudConfig += fmt.Sprintf("echo %s >> $USER_DIR/.ssh/authorized_keys \n", key)
		}
	}

	return cloudConfig
}

func CloudConfigCoreOS(keys []string) string {
	cloudConfig := `#cloud-config

coreos:
    units:
        - name: ovs.service
          command: start
          content: |
            [Unit]
            Description=OVS
            [Service]
            ExecStart=/sbin/modprobe openvswitch
            ExecStartPost=/sbin/modprobe vport_geneve

        - name: docker.service
          command: start
          content: |
            [Unit]
            Description=docker
            [Service]
            ExecStart=/usr/bin/docker daemon --bridge=none \
            -H $private_ipv4:2375 -H unix:///var/run/docker.sock \
            --cluster-store=etcd://127.0.0.1:2379 --cluster-advertise=$private_ipv4:0
            ExecStartPost=-/usr/bin/mkdir -p /var/run/netns

        - name: minion.service
          command: start
          content: |
            [Unit]
            Description=DI Minion
            After=docker.service
            Requires=docker.service

            [Service]
            TimeoutSec=1000
            ExecStartPre=-/usr/bin/docker kill minion
            ExecStartPre=-/usr/bin/docker rm minion
            ExecStartPre=/usr/bin/docker pull %s
            ExecStart=/usr/bin/docker run --net=host --name=minion --privileged \
            -v /var/run/docker.sock:/var/run/docker.sock %s

`
	cloudConfig = fmt.Sprintf(cloudConfig, minionImage, minionImage)

	if len(keys) > 0 {
		cloudConfig += "ssh_authorized_keys:\n"
		for _, key := range keys {
			cloudConfig += fmt.Sprintf("    - \"%s\"\n", key)
		}
	}

	return cloudConfig
}
