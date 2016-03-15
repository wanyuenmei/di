package network

import (
	"fmt"
	"reflect"
	"testing"
)

func TestListIP(t *testing.T) {
	oldIpExecVerbose := ipExecVerbose
	defer func() { ipExecVerbose = oldIpExecVerbose }()
	ipExecVerbose = func(namespace, format string, args ...interface{}) (
		stdout, stderr []byte, err error) {
		return []byte(ips()), nil, nil
	}
	actual, _ := listIP("")
	exp := []string{"10.0.2.15/24", "fe80::a00:27ff:fe9b:594e/64"}
	if !reflect.DeepEqual(actual, exp) {
		t.Error(fmt.Sprintf("Generated wrong IPs."+
			"\nExpected:\n%s\n\nGot:\n%s\n", exp, actual))
	}
}

func TestGetDefaultGateway(t *testing.T) {
	oldIpExecVerbose := ipExecVerbose
	defer func() { ipExecVerbose = oldIpExecVerbose }()
	ipExecVerbose = func(namespace, format string, args ...interface{}) (
		stdout, stderr []byte, err error) {
		return []byte(routes()), nil, nil
	}
	actual, _ := getDefaultGateway("")
	exp := "10.0.2.2"
	if actual != exp {
		t.Error(fmt.Sprintf("Generated wrong default gateway."+
			"\nExpected:\n%s\n\nGot:\n%s\n", exp, actual))
	}
}

func ips() string {
	return `2: eth0: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc pfifo_fast state UP group default qlen 1000
    link/ether 08:00:27:9b:59:4e brd ff:ff:ff:ff:ff:ff
    inet 10.0.2.15/24 brd 10.0.2.255 scope global eth0
       valid_lft forever preferred_lft forever
    inet6 fe80::a00:27ff:fe9b:594e/64 scope link
       valid_lft forever preferred_lft forever`
}

func routes() string {
	return `default via 10.0.2.2 dev eth0
	10.0.2.0/24 dev eth0  proto kernel  scope link  src 10.0.2.15
	192.168.162.0/24 dev eth1  proto kernel  scope link  src 192.168.162.162`
}
