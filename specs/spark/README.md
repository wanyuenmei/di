# Quilt: Spark
This document describes how to run Apache Spark on Quilt and pass it a job at
boot time. Specifically, we'll be running the SparkPI example to calculate the
value of π on our Quilt Spark cluster.

## Configuring QUILT_PATH
Quilt uses the `QUILT_PATH` environment variable to locate packages.

Execute the following export commands in your terminal to allow Quilt to locate
Quilt Spark:
```bash
quilt=$GOPATH/src/github.com/NetSys/quilt
export QUILT_PATH=$quilt/specs:$quilt/specs/spark:$quilt/specs/zookeeper
```
It would be a good idea to add these commands to your `.bashrc` (or appropriate
dotfile) so that they don't have to be run again.

## SparkPi
The example SparkPi program distributes the computationally-intensive task of
calculating Pi over several machines in a computer cluster.

Our [sparkPI.spec](sparkPI.spec) Stitch specification simplifies the
task of setting up the infrastructure required to run this Spark job.

### Configure SSH authentication
Quilt-managed Machines use public key authentication to control SSH access.
To read the result of the Spark job, we will need to access the Master VM.

If you would like to use `githubKey` authentication, open
`specs/spark/sparkPI.spec` and fill in
`(define githubKey "<YOUR_GITHUB_USERNAME>")` appropriately.

For instructions on configuring a user-supplied public key and more information
on configuring Quilt SSH authentication, see
[GettingStarted.md](../../docs/GettingStarted.md#set-up-your-ssh-authentication).

### Choose Namespace
Running two Quilt instances with the same Namespace is not supported.
If you are sharing a computing cluster with others, it would be a good idea to
change `(define Namespace "CHANGE_ME")` to a different name.

### Build `sparkPI.spec`
Execute `quilt $GOPATH/github.com/NetSys/Quilt/specs/spark/sparkPI.spec` to
build this Stitch specification.

Quilt will now begin provisioning several VMs on your cloud provider. Five VMs
will serve as Workers, and one will be the Master.

It will take a bit for the VMs to boot up for Quilt to configure the network,
and for Docker containers to be initialized. The following output reports that
the Master's public IP is `54.183.162.8`:
```
INFO [Jun  8 10:41:09.268] Successfully booted machines.
INFO [Jun  8 10:41:20.820] db.Machine:
    Machine-2{ClusterID=1, Role=Master, Provider=Amazon, Region=us-west-1, Size=m4.2xlarge, DiskSize=32, CloudID=sir-041ecz1b, PublicIP=54.183.162.8, PrivateIP=172.31.11.161}
    Machine-8{ClusterID=1, Role=Worker, Provider=Amazon, Region=us-west-1, Size=m4.2xlarge, DiskSize=32, CloudID=sir-041f2tpn, PublicIP=54.67.99.218, PrivateIP=172.31.15.97}

[truncated]
```

A "New connection" message in the console output indicates that new VM is fully
booted and has began communicating with Quilt:

```
INFO [Jun  8 10:44:10.523] New connection.
    machine=Machine-2{ClusterID=1, Role=Master, Provider=Amazon, Region=us-west-1, Size=m4.2xlarge, DiskSize=32, CloudID=sir-041ecz1b, PublicIP=54.183.162.8, PrivateIP=172.31.11.161}
```

Once you see the "New connection" message, you can connect to the Machines with the command
`ssh quilt@<PUBLIC_IP>`.

### Inspect Docker Containers
Docker Swarm has a global view of all the containers in the cluster.  To make
it easy to access, the Master nodes have a command line utility, `swarm`, that
directs docker commands to the swarm cluster.  For example, to list all of the
active Docker containers in the cluster use `swarm ps`.  For example:
```
quilt@ip-172-31-11-161:~$ swarm ps
CONTAINER ID        IMAGE                        COMMAND                  CREATED             STATUS              PORTS               NAMES
b4fec2b9950b        quilt/spark                  "run worker"             8 minutes ago       Up 8 minutes                            ip-172-31-7-86/jovial_poincare
b532520449b9        quilt/spark                  "run worker"             8 minutes ago       Up 8 minutes                            ip-172-31-1-237/jovial_wright
d41dc1480707        quilt/spark                  "run worker"             8 minutes ago       Up 8 minutes                            ip-172-31-10-179/amazing_torvalds
c29d3201c633        quilt/spark                  "run worker"             8 minutes ago       Up 8 minutes                            ip-172-31-15-97/small_leakey
0cc00855552e        quilt/spark                  "run worker"             8 minutes ago       Up 8 minutes                            ip-172-31-9-185/tiny_pike
3f4fcbb962a5        quilt/spark                  "run master"             8 minutes ago       Up 8 minutes                            ip-172-31-8-88/berserk_ptolemy

[truncated]
```

If the `quilt/spark` containers are not running, it means the `quilt/spark`
containers are still being downloaded.

### Recovering Pi
Once our Master Spark container is up, we can connect to it to find the results of
our SparkPi job! The results are located in the log of the Master container. In
the output above, our Master container's name is `ip-172-31-7-86/jovial_poincare`.

Execute `swarm logs <MASTER_CONTAINER_NAME>`. After
scrolling through Spark's info logging, we will find the result of SparkPi:

```
16/06/08 18:49:42 INFO TaskSchedulerImpl: Removed TaskSet 0.0, whose tasks have all completed, from pool
16/06/08 18:49:42 INFO DAGScheduler: ResultStage 0 (reduce at SparkPi.scala:36) finished in 0.381 s
16/06/08 18:49:42 INFO DAGScheduler: Job 0 finished: reduce at SparkPi.scala:36, took 0.525937 s
Pi is roughly 3.13918
16/06/08 18:49:42 INFO SparkUI: Stopped Spark web UI at http://10.0.254.144:4040
16/06/08 18:49:42 INFO MapOutputTrackerMasterEndpoint: MapOutputTrackerMasterEndpoint stopped!
16/06/08 18:49:42 INFO MemoryStore: MemoryStore cleared
16/06/08 18:49:42 INFO BlockManager: BlockManager stopped
```

**Note:** The Spark cluster is now up and usable. You can run the interactive
spark-shell by exec-ing it in the Master Spark container:
`swarm exec -it <MASTER_CONTAINER_NAME> spark-shell`
