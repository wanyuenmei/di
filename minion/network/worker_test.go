package network

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/NetSys/di/db"
)

func TestNoConnections(t *testing.T) {
	labels, connections := defaultLabelsConnections()
	dbc := db.Container{
		ID:      1,
		SchedID: "abcdefghijklmnopqrstuvwxyz",
		IP:      "1.1.1.1",
		Labels:  []string{"green"},
	}

	actual := generateEtcHosts(dbc, labels, connections)
	exp := "1.1.1.1         abcdefghijkl" + localhosts()

	if exp != actual {
		t.Error(fmt.Sprintf("Generated wrong basic /etc/hosts."+
			"\nExpected:\n%s\n\nGot:\n%s\n", exp, actual))
	}
}

func TestImplementsSingleLabel(t *testing.T) {
	labels, connections := defaultLabelsConnections()
	dbc := db.Container{
		ID:      2,
		SchedID: "abcdefghijklmnopqrstuvwxyz",
		IP:      "1.1.1.1",
		Labels:  []string{"red"},
	}

	actual := generateEtcHosts(dbc, labels, connections)
	exp := `1.1.1.1         abcdefghijkl
10.0.0.2        blue.di
10.0.0.3        green.di` + localhosts()

	if exp != actual {
		t.Error(fmt.Sprintf("Generated wrong single label /etc/hosts."+
			"\nExpected:\n%s\n\nGot:\n%s\n", exp, actual))
	}
}

func TestImplementsMultipleLabels(t *testing.T) {
	labels, connections := defaultLabelsConnections()
	dbc := db.Container{
		ID:      3,
		SchedID: "abcdefghijklmnopqrstuvwxyz",
		IP:      "1.1.1.1",
		Labels:  []string{"red", "blue"},
	}

	actual := generateEtcHosts(dbc, labels, connections)
	exp := `1.1.1.1         abcdefghijkl
10.0.0.1        red.di
10.0.0.2        blue.di
10.0.0.3        green.di` + localhosts()

	if exp != actual {
		t.Error(fmt.Sprintf("Generated wrong multi-label /etc/hosts"+
			"\nExpected:\n%s\n\nGot:\n%s\n", exp, actual))
	}
}

// Both red and blue connect to green. Make sure that green.di only appears once in /etc/hosts.
func TestDuplicateConnections(t *testing.T) {
	labels, connections := defaultLabelsConnections()
	dbc := db.Container{
		ID:      4,
		SchedID: "abcdefghijklmnopqrstuvwxyz",
		IP:      "1.1.1.1",
		Labels:  []string{"red", "blue"},
	}

	connections["blue"] = append(connections["blue"], "green")

	actual := generateEtcHosts(dbc, labels, connections)
	exp := `1.1.1.1         abcdefghijkl
10.0.0.1        red.di
10.0.0.2        blue.di
10.0.0.3        green.di` + localhosts()

	if exp != actual {
		t.Error(fmt.Sprintf(
			"Generated wrong /etc/hosts for duplicate connections."+
				"\nExpected:\n%s\n\nGot:\n%s\n", exp, actual))
	}
}

func TestMakeIPRule(t *testing.T) {
	inp := "-A INPUT -p tcp -i eth0 -m multiport --dports 465,110,995 -j ACCEPT"
	rule, _ := makeIPRule(inp)
	expCmd := "-A"
	expChain := "INPUT"
	expOpts := "--dports 110,465,995 -i eth0 -j ACCEPT -m multiport -p tcp"

	if rule.cmd != expCmd {
		t.Error(fmt.Sprintf(
			"Bad ipRule command."+
				"\nExpected:\n%s\n\nGot:\n%s\n", expCmd, rule.cmd))
	}

	if rule.chain != expChain {
		t.Error(fmt.Sprintf(
			"Bad ipRule chain."+
				"\nExpected:\n%s\n\nGot:\n%s\n", expChain, rule.chain))
	}

	if rule.opts != expOpts {
		t.Error(fmt.Sprintf(
			"Bad ipRule options."+
				"\nExpected:\n%s\n\nGot:\n%s\n", expOpts, rule.opts))
	}

	inp = "-A POSTROUTING -s 10.0.3.0/24 ! -d 10.0.3.0/24 -j MASQUERADE"
	rule, _ = makeIPRule(inp)
	expCmd = "-A"
	expChain = "POSTROUTING"
	expOpts = "! -d 10.0.3.0/24 -j MASQUERADE -s 10.0.3.0/24"

	if rule.cmd != expCmd {
		t.Error(fmt.Sprintf(
			"Bad ipRule command."+
				"\nExpected:\n%s\n\nGot:\n%s\n", expCmd, rule.cmd))
	}

	if rule.chain != expChain {
		t.Error(fmt.Sprintf(
			"Bad ipRule chain."+
				"\nExpected:\n%s\n\nGot:\n%s\n", expChain, rule.chain))
	}

	if rule.opts != expOpts {
		t.Error(fmt.Sprintf(
			"Bad ipRule options."+
				"\nExpected:\n%s\n\nGot:\n%s\n", expOpts, rule.opts))
	}
}

func TestGenerateCurrentRoutes(t *testing.T) {
	oldIpExecVerbose := ipExecVerbose
	defer func() { ipExecVerbose = oldIpExecVerbose }()
	ipExecVerbose = func(namespace, format string, args ...interface{}) (
		stdout, stderr []byte, err error) {
		return []byte(routes()), nil, nil
	}
	actual, _ := generateCurrentRoutes("")

	exp := []route{
		{
			ip:        "10.0.2.0/24",
			dev:       "eth0",
			isDefault: false,
		},
		{
			ip:        "192.168.162.0/24",
			dev:       "eth1",
			isDefault: false,
		},
		{
			ip:        "10.0.2.2",
			dev:       "eth0",
			isDefault: true,
		},
	}

	if !(reflect.DeepEqual(actual, exp)) {
		t.Error(fmt.Sprintf("Generated wrong routes."+
			"\nExpected:\n%+v\n\nGot:\n%+v\n", exp, actual))
	}
}

func TestGenerateCurrentNatRules(t *testing.T) {
	oldShVerbose := shVerbose
	defer func() { shVerbose = oldShVerbose }()
	shVerbose = func(format string, args ...interface{}) (
		stdout, stderr []byte, err error) {
		return []byte(rules()), nil, nil
	}

	actual, _ := generateCurrentNatRules()
	exp := []ipRule{
		{
			cmd:   "-P",
			chain: "POSTROUTING ACCEPT",
		},
		{
			cmd:   "-N",
			chain: "DOCKER",
		},
		{
			cmd:   "-A",
			chain: "POSTROUTING",
			opts:  "-j MASQUERADE -o eth0 -s 10.0.0.0/8,11.0.0.0/8",
		},
		{
			cmd:   "-A",
			chain: "POSTROUTING",
			opts:  "! -d 10.0.3.0/24 -j MASQUERADE -s 10.0.3.0/24",
		},
	}

	if !(reflect.DeepEqual(actual, exp)) {
		t.Error(fmt.Sprintf("Generated wrong routes."+
			"\nExpected:\n%+v\n\nGot:\n%+v\n", exp, actual))
	}
}

func defaultLabelsConnections() (map[string]string, map[string][]string) {

	labels := map[string]string{
		"red":   "10.0.0.1",
		"blue":  "10.0.0.2",
		"green": "10.0.0.3",
	}

	connections := map[string][]string{
		"red":  {"blue", "green"},
		"blue": {"red"},
	}

	return labels, connections
}

func localhosts() string {
	return `
127.0.0.1       localhost
::1             localhost ip6-localhost ip6-loopback
fe00::0         ip6-localnet
ff00::0         ip6-mcastprefix
ff02::1         ip6-allnodes
ff02::2         ip6-allrouters
`
}

func routes() string {
	return `default via 10.0.2.2 dev eth0
	10.0.2.0/24 dev eth0  proto kernel  scope link  src 10.0.2.15
	192.168.162.0/24 dev eth1  proto kernel  scope link  src 192.168.162.162`
}

func rules() string {
	return `-P POSTROUTING ACCEPT
-N DOCKER
-A POSTROUTING -s 11.0.0.0/8,10.0.0.0/8 -o eth0 -j MASQUERADE
-A POSTROUTING -s 10.0.3.0/24 ! -d 10.0.3.0/24 -j MASQUERADE`
}
