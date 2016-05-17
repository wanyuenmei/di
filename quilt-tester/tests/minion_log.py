#!/usr/bin/env python
import subprocess
import string
import sys

def sh(cmd):
    return subprocess.check_output(cmd, shell=True)

def log(container):
    header = string.join(["=" for _ in range(50)], "")

    try:
        output = sh("docker logs %s" % container)
    except subprocess.CalledProcessError as e:
        output = "%s" % e

    print "\n%s %s %s\n%s" % (header, container, header, output)
    sys.stdout.flush()

log("minion")
log("etcd")
log("swarm")
log("ovs-vswitchd")
log("ovsdb-server")
log("ovn-controller")
log("ovn-northd")
