package provider

import (
	"fmt"
	"strings"
)

const (
	quiltImage = "quilt/quilt:latest"
)

var cloudConfigFormat = `#!/bin/bash

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
	# If getting the AWS internal IP works, then use that; otherwise, manually
	# parse it ourselves.
	PRIVATE_IPv4="$(curl -s --connect-timeout 5 http://instance-data/latest/meta-data/local-ipv4)"
	if [ -z "$PRIVATE_IPv4" ]; then
		PRIVATE_IPv4="$(ip address show eth1 | grep 'inet ' | sed -e 's/^.*inet //' -e 's/\/.*$//' | tr -d '\n')"
	fi
	if [ -z "$PRIVATE_IPv4" ]; then
		PRIVATE_IPv4="$(hostname -i)"
	fi

	mkdir -p /etc/systemd/system/docker.service.d

	echo "docker -H tcp://${PRIVATE_IPv4}:2377 \$@" > /usr/bin/swarm
	chmod 755 /usr/bin/swarm

	cat <<- EOF > /etc/systemd/system/docker.service.d/override.conf
	[Unit]
	Description=docker

	[Service]
	# The below empty ExecStart deletes the official one installed by docker daemon.
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
	Description=Quilt Minion
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
	-v /proc:/hostproc:ro -v /var/run/netns:/var/run/netns:rw %[1]s \
	/minion

	[Install]
	WantedBy=multi-user.target
	EOF
}

install_docker() {
	echo "deb https://apt.dockerproject.org/repo ubuntu-%[3]s main" > /etc/apt/sources.list.d/docker.list
	apt-get update
	apt-get install docker-engine=1.11.2-0~%[3]s -y --force-yes
	systemctl stop docker.service
}

setup_user() {
	user=$1
	ssh_keys=$2
	sudo groupadd $user
	sudo useradd $user -s /bin/bash -g $user
	sudo usermod -aG sudo $user
	# Allow the user to use docker without sudo
	sudo usermod -aG docker $user

	user_dir=/home/$user

	# Create dirs and files with correct users and permissions
	install -d -o $user -m 744 $user_dir
	install -d -o $user -m 700 $user_dir/.ssh
	install -o $user -m 600 /dev/null $user_dir/.ssh/authorized_keys
	printf "$ssh_keys" >> $user_dir/.ssh/authorized_keys
	printf "$user ALL = (ALL) NOPASSWD: ALL\n" >> /etc/sudoers
}

echo -n "Start Boot Script: " >> /var/log/bootscript.log
date >> /var/log/bootscript.log

export DEBIAN_FRONTEND=noninteractive

install_docker
initialize_ovs
initialize_docker
initialize_minion

ssh_keys="%[2]s"
setup_user quilt "$ssh_keys"

# Reload because we replaced the docker.service provided by the package
systemctl daemon-reload

# Enable our services to run on boot
systemctl enable {ovs,docker,minion}.service

# Start our services
systemctl restart {ovs,docker,minion}.service

echo -n "Completed Boot Script: " >> /var/log/bootscript.log
date >> /var/log/bootscript.log
    `

func cloudConfigUbuntu(keys []string, ubuntuVersion string) string {
	keyStr := strings.Join(keys, "\n")
	return fmt.Sprintf(cloudConfigFormat, quiltImage, keyStr, ubuntuVersion)
}
