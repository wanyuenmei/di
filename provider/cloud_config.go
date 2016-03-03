package provider

import "fmt"

const (
	minionImage = "quay.io/netsys/di-minion:latest"
)

func cloudConfigUbuntu(keys []string, user string, ubuntuVersion string) string {
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
	ExecStartPre=-/usr/bin/mkdir -p /var/run/netns
	ExecStartPre=-/usr/bin/docker kill minion
	ExecStartPre=-/usr/bin/docker rm minion
	ExecStartPre=/usr/bin/docker pull %[1]s
	ExecStart=/usr/bin/docker run --net=host --name=minion --privileged \
	-v /var/run/docker.sock:/var/run/docker.sock \
	-v /proc:/hostproc:ro -v /var/run/netns:/var/run/netns:rw %[1]s

	[Install]
	WantedBy=multi-user.target
	EOF
}

install_docker() {
	# Disable default sources list since we don't use them anyways
	mv /etc/apt/sources.list /etc/apt/sources.list.bak

	echo "deb https://apt.dockerproject.org/repo ubuntu-%[3]s main" > /etc/apt/sources.list.d/docker.list
	apt-get update
	apt-get install docker-engine=1.9.1-0~%[3]s -y --force-yes
	systemctl stop docker.service
}

echo -n "Start Boot Script: " >> /var/log/bootscript.log
date >> /var/log/bootscript.log

USER_DIR=/home/%[2]s
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
install -d -o %[2]s -m 700 $USER_DIR/.ssh
install -o %[2]s -m 600 /dev/null $USER_DIR/.ssh/authorized_keys

# allow the user to use docker without sudo
usermod -aG docker %[2]s

echo -n "Completed Boot Script: " >> /var/log/bootscript.log
date >> /var/log/bootscript.log
    `
	cloudConfig = fmt.Sprintf(cloudConfig, minionImage, user, ubuntuVersion)

	if len(keys) > 0 {
		for _, key := range keys {
			cloudConfig += fmt.Sprintf("echo %s >> $USER_DIR/.ssh/authorized_keys \n", key)
		}
	}

	return cloudConfig
}

func cloudConfigCoreOS(keys []string) string {
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

        - name: minion.service
          command: start
          content: |
            [Unit]
            Description=DI Minion
            After=docker.service
            Requires=docker.service

            [Service]
            TimeoutSec=1000
            ExecStartPre=-/usr/bin/mkdir -p /var/run/netns
            ExecStartPre=-/usr/bin/docker kill minion
            ExecStartPre=-/usr/bin/docker rm minion
            ExecStartPre=/usr/bin/docker pull %s
            ExecStart=/usr/bin/docker run --net=host --name=minion --privileged \
            -v /var/run/docker.sock:/var/run/docker.sock \
            -v /proc:/hostproc:ro -v /var/run/netns:/var/run/netns:rw

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
