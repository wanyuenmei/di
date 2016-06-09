# GETTING STARTED
This guide explains how to install Quilt, and also serves as a
brief, hands-on introduction to some Quilt basics.

## Install Go
Quilt supports Go version 1.5 or later.

Find Go using your package manager or on the [Golang website] (https://golang.org/doc/install).

### Setup $GOPATH
We recommend reading the overview to Go workplaces [here](https://golang.org/doc/code.html).

Before installing Quilt, you'll need to set up your GOPATH. Assuming the root of
your Go workspace will be `~/gowork`, execute the following `export` commands in
your terminal to set up your GOPATH.
```
export GOPATH=~/gowork
export PATH=$PATH:$GOPATH/bin
```
It would be a good idea to add these commands to your `.bashrc` so that they do
not have to be run again.

## Download and Install Quilt
Clone the repository into your Go workspace: `go get github.com/NetSys/quilt`.

This command also automatically installs Quilt. If the installation was
successful, then the `quilt` command should execute successfully in your shell.

## Configure Amazon Web Service Settings
If you'd like to use AWS as Quilt's cloud provider, create the file
`~/.aws/credentials` with the following contents:
```
[default]
aws_access_key_id = <YOUR_ID>
aws_secret_access_key = <YOUR_SECRET_KEY>
```

Here are instructions for
[finding your AWS access keys](http://docs.aws.amazon.com/cli/latest/userguide/cli-chap-getting-set-up.html#cli-signup).

## Your First Quilt-managed Infrastructure
We suggest you read `example.spec` to understand the infrastructure defined by
this Stitch.

### Configure `example.spec`
#### Set Up Your SSH Authentication
Quilt-managed Machines use public key authentication to control SSH access. Stitch
defines two Machine attributes for configuring SSH authentication, `githubKey`
and `sshkey`.

##### githubKey
A Machine with the `githubKey` attribute uses your public keys from GitHub
to configure SSH authentication. If you can access GitHub repositories through
SSH, then you can also SSH into a `githubKey`-configured Machine.

If you would like to use `githubKey` authentication, open `example.spec` and
fill in `(define githubKey "<YOUR_GITHUB_USERNAME>")` appropriately.

##### sshkey
Machines with the `sshkey` attribute use a user-supplied public key to configure
SSH authentication. This attribute is useful for users who want to use an ssh key
that isn't stored on GitHub.

To use `sshkey` authentication, open `example.spec` for editting.

Replace `(define githubKey
"<YOUR_GITHUB_USERNAME>")` with `(define sshkey <YOUR_PUBLIC_KEY>)`.

`<YOUR_PUBLIC_KEY>` should be a string with the contents of your
`~/.ssh/id_rsa.pub`, e.g.: `(define sshkey "ssh-rsa
AAAAB3NzaC1yc2EAAAADAQABAAABAQ shwang@deepmuse.space")`

#### Choose Namespace
Running two Quilt instances with the same Namespace is not supported.
If you are sharing a computing cluster with others, it would be a good idea to
change `(define Namespace "example")` to a different name.

### Building `example.spec`
While in the `$GOPATH/src/github.com/NetSys/quilt/` directory, execute `quilt
example.spec`. Quilt will set up several Ubuntu VMs on your cloud provider as
Workers, and these Workers will host Nginx Docker containers.


### Accessing the Worker VM
It will take a while for the VMs to boot up, for Quilt to configure the network,
and for Docker containers to be initialized. "New connection" message in the console
output indicates that new VM is fully booted and has began communicating with
Quilt.

The public IP of the Worker VM can be deduced from the console output. The
following output shows the Worker VM's public IP to be 52.39.213.45:
```
INFO [Jun  7 17:24:43.660] New connection.
machine=Machine-3{ClusterID=1, Role=Worker, Provider=Amazon, Region=us-west-2, Size=m3.medium, DiskSize=32, CloudID=sir-03hg1gw1, PublicIP=52.39.213.45, PrivateIP=172.31.6.254}
INFO [Jun  7 17:24:43.660] New connection.
machine=Machine-2{ClusterID=1, Role=Master, Provider=Amazon, Region=us-west-2, Size=m3.medium, DiskSize=32, CloudID=sir-03hdcezk, PublicIP=52.25.237.134, PrivateIP=172.31.1.198}
```

Run `ssh quilt@<WORKER_PUBLIC_IP>` to access a privileged shell on the Worker VM.

### Inspecting Docker Containers on the Worker VM
You can run `docker ps` to list the containers running on your Worker VM.

```
quilt@ip-172-31-1-198:~$ docker ps
CONTAINER ID        IMAGE                        COMMAND                  CREATED             STATUS              PORTS               NAMES
4ec5926bfbab        quilt/ovs                    "run ovn-northd"         2 hours ago         Up 2 hours                              ovn-northd
70b785769fc8        swarm:1.2.3                  "/swarm manage --repl"   2 hours ago         Up 2 hours                              swarm
855e6ff38345        quilt/ovs                    "run ovsdb-server"       2 hours ago         Up 2 hours                              ovsdb-server
fb0f44812f30        quay.io/coreos/etcd:v2.3.6   "/etcd --name=master-"   2 hours ago         Up 2 hours                              etcd
79ca96065912        quilt/quilt:latest           "/minion"                2 hours ago         Up 2 hours                              minion
```

Any docker containers defined in a Stitch specification are placed on one of
your Worker VMs.  In addition to these user-defined containers, Quilt also
places several support containers on each VM. Among these support containers is
`minion`, which locally manages Docker and allows Quilt VMs to talk to each
other and your local computer.

### Loading the Nginx Webpage
By default, Quilt-managed containers are disconnected from the public internet
and isolated from one another.
The final line of `example.spec` opens port 80 on the Nginx container to the
outside world.

From your browser via `http://<WORKER_PUBLIC_IP>`, or on the command-line via
`curl <WORKER_PUBLIC_IP>`, you can load the Nginx welcome page served by your
Quilt cluster.

## Next Steps: Starting Spark
A starter Spark example to explore is [SparkPI](specs/spark/).
