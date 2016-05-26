# Open vSwitch
Containerized versions of Open vSwitch built originally for use by
[Quilt](http://quilt.io).  Despite their original intent, there is nothing Quilt
specific about them, thus they should be generally useful.

## Using
The quilt/ovs container can run `ovs-vswitchd`, `ovsdb-server`, `ovn-northd`,
and `ovn-controller`.  It chooses which flavor to boot based on a command line
argument (examples below).

The containers have several initialization requirements that make running them
slightly different than typical docker containers.  The most important
of which is the necessity that all containers run on the host's network stack.
For example, the proper way to boot `ovsdb-server`, the simplest of the
containers, is as follows:

    docker run -itd --net=host --name=ovsdb-server quilt/ovs ovsdb-server

All of the other flavors require several volumes containing the OVS database,
logs, and configuration files.  These must be mounted from a running
ovsdb-server container as follows:

    docker run -itd --net=host --volumes-from=ovsdb-server quilt/ovs ovn-northd

    docker run -itd --net=host --volumes-from=ovsdb-server quilt/ovs ovn-controller

Finally, Open vSwitch needs to interact with the OVS kernel module in the host,
and therefore must be run in privileged mode.

    docker run -itd --net=host --volumes-from=ovsdb-server --privileged quilt/ovs ovs-vswitchd
