#!/usr/bin/env python
import subprocess
import string

def sh(cmd):
    return subprocess.check_output(cmd, shell=True)

def log(container):
    header = string.join(["=" for _ in range(50)], "")
    print "%s %s %s" % (header, container, header)

    try:
        output = sh("docker logs %s" % container)
    except subprocess.CalledProcessError as e:
        output = "%s" % e

    print "%s\n" % output

log("minion")
log("etcd")
log("swarm")
log("ovs-vswitchd")
log("ovsdb-server")
log("ovn-controller")
log("ovn-northd")
