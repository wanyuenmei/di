package network

import (
	"reflect"
	"testing"
)

func TestListIP(t *testing.T) {
	oldIPExecVerbose := ipExecVerbose
	defer func() { ipExecVerbose = oldIPExecVerbose }()
	ipExecVerbose = func(namespace, format string, args ...interface{}) (
		stdout, stderr []byte, err error) {
		return []byte(ips()), nil, nil
	}
	actual, _ := listIP("", innerVeth)
	exp := []string{"10.0.2.15/24", "fe80::a00:27ff:fe9b:594e/64"}
	if !reflect.DeepEqual(actual, exp) {
		t.Errorf("Generated wrong IPs.\nExpected:\n%s\n\nGot:\n%s\n",
			exp, actual)
	}
}

func TestLinkIsUp(t *testing.T) {
	oldIPExecVerbose := ipExecVerbose
	defer func() { ipExecVerbose = oldIPExecVerbose }()
	ipExecVerbose = func(namespace, format string, args ...interface{}) (
		stdout, stderr []byte, err error) {
		return []byte(ips()), nil, nil
	}

	exp := true
	isUp, _ := linkIsUp("", "eth0")
	if exp != isUp {
		t.Errorf("Got wrong link state.\nExpected:\n%t\n\nGot:\n%t\n",
			exp, isUp)
	}

	exp = false
	isUp, _ = linkIsUp("", "eth1")
	if exp != isUp {
		t.Errorf("Got wrong link state.\nExpected:\n%t\n\nGot:\n%t\n", exp,
			isUp)
	}
}

func ips() string {
	return `2: eth0: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc pfifo_fast state UP group default qlen 1000
    link/ether 08:00:27:9b:59:4e brd ff:ff:ff:ff:ff:ff
    inet 10.0.2.15/24 brd 10.0.2.255 scope global eth0
		valid_lft forever preferred_lft forever
    inet6 fe80::a00:27ff:fe9b:594e/64 scope link
		valid_lft forever preferred_lft forever
    6: eth1: <BROADCAST,MULTICAST> mtu 1500 qdisc noop state DOWN group default
		link/ether 0e:9f:0c:21:65:4a brd ff:ff:ff:ff:ff:ff`
}
