#!/usr/bin/env python
import subprocess
import string

def sh(cmd):
    return subprocess.check_output(cmd, shell=True)

def title(t):
    header = string.join(["=" for _ in range(50)], "")
    print "%s %s %s" % (header, t, header)

title("Minion")
print sh("docker logs minion")

title("Etcd")
print sh("docker logs etcd")

title("Swarm")
print sh("docker logs swarm")

title("ovs-vswitchd")
print sh("docker logs ovs-vswitchd")

title("ovsdb-server")
print sh("docker logs ovsdb-server")

title("ovn-controller")
print sh("docker logs ovn-controller")

title("ovn-northd")
print sh("docker logs ovn-northd")
