package network

import (
	"reflect"
	"testing"

	"github.com/NetSys/quilt/db"
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
		t.Errorf("Generated wrong basic /etc/hosts."+
			"\nExpected:\n%s\n\nGot:\n%s\n", exp, actual)
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
10.0.0.2        blue.q
10.0.0.3        green.q` + localhosts()

	if exp != actual {
		t.Errorf("Generated wrong single label /etc/hosts."+
			"\nExpected:\n%s\n\nGot:\n%s\n", exp, actual)
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
10.0.0.1        red.q
10.0.0.2        blue.q
10.0.0.3        green.q` + localhosts()

	if exp != actual {
		t.Errorf("Generated wrong multi-label /etc/hosts"+
			"\nExpected:\n%s\n\nGot:\n%s\n", exp, actual)
	}
}

// Both red and blue connect to green. Make sure that green.q only appears once in
// /etc/hosts.
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
10.0.0.1        red.q
10.0.0.2        blue.q
10.0.0.3        green.q` + localhosts()

	if exp != actual {
		t.Errorf("Generated wrong /etc/hosts for duplicate connections."+
			"\nExpected:\n%s\n\nGot:\n%s\n", exp, actual)
	}
}

func TestMakeIPRule(t *testing.T) {
	inp := "-A INPUT -p tcp -i eth0 -m multiport --dports 465,110,995 -j ACCEPT"
	rule, _ := makeIPRule(inp)
	expCmd := "-A"
	expChain := "INPUT"
	expOpts := "-p tcp -i eth0 -m multiport --dports 465,110,995 -j ACCEPT"

	if rule.cmd != expCmd {
		t.Errorf("Bad ipRule command.\nExpected:\n%s\n\nGot:\n%s\n",
			expCmd, rule.cmd)
	}

	if rule.chain != expChain {
		t.Errorf("Bad ipRule chain.\nExpected:\n%s\n\nGot:\n%s\n",
			expChain, rule.chain)
	}

	if rule.opts != expOpts {
		t.Errorf("Bad ipRule options.\nExpected:\n%s\n\nGot:\n%s\n",
			expOpts, rule.opts)
	}

	inp = "-A POSTROUTING -s 10.0.3.0/24 ! -d 10.0.3.0/24 -j MASQUERADE"
	rule, _ = makeIPRule(inp)
	expCmd = "-A"
	expChain = "POSTROUTING"
	expOpts = "-s 10.0.3.0/24 ! -d 10.0.3.0/24 -j MASQUERADE"

	if rule.cmd != expCmd {
		t.Errorf("Bad ipRule command.\nExpected:\n%s\n\nGot:\n%s\n",
			expCmd, rule.cmd)
	}

	if rule.chain != expChain {
		t.Errorf("Bad ipRule chain.\nExpected:\n%s\n\nGot:\n%s\n",
			expChain, rule.chain)
	}

	if rule.opts != expOpts {
		t.Errorf("Bad ipRule options.\nExpected:\n%s\n\nGot:\n%s\n",
			expOpts, rule.opts)
	}

	inp = "-A PREROUTING -i eth0 -p tcp --dport 80 -j DNAT --to-destination 10.31.0.23:80"
	rule, _ = makeIPRule(inp)
	expCmd = "-A"
	expChain = "PREROUTING"
	expOpts = "-i eth0 -p tcp --dport 80 -j DNAT --to-destination 10.31.0.23:80"

	if rule.cmd != expCmd {
		t.Errorf("Bad ipRule command.\nExpected:\n%s\n\nGot:\n%s\n",
			expCmd, rule.cmd)
	}

	if rule.chain != expChain {
		t.Errorf("Bad ipRule chain.\nExpected:\n%s\n\nGot:\n%s\n",
			expChain, rule.chain)
	}

	if rule.opts != expOpts {
		t.Errorf("Bad ipRule options.\nExpected:\n%s\n\nGot:\n%s\n",
			expOpts, rule.opts)
	}
}

func TestGenerateCurrentRoutes(t *testing.T) {
	oldIPExecVerbose := ipExecVerbose
	defer func() { ipExecVerbose = oldIPExecVerbose }()
	ipExecVerbose = func(namespace, format string, args ...interface{}) (
		stdout, stderr []byte, err error) {
		return []byte(routes()), nil, nil
	}
	actual, _ := generateCurrentRoutes("")

	exp := routeSlice{
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
		t.Errorf("Generated wrong routes.\nExpected:\n%+v\n\nGot:\n%+v\n",
			exp, actual)
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
	exp := ipRuleSlice{
		{
			cmd:   "-P",
			chain: "POSTROUTING",
			opts:  "ACCEPT",
		},
		{
			cmd:   "-N",
			chain: "DOCKER",
		},
		{
			cmd:   "-A",
			chain: "POSTROUTING",
			opts:  "-s 11.0.0.0/8,10.0.0.0/8 -o eth0 -j MASQUERADE",
		},
		{
			cmd:   "-A",
			chain: "POSTROUTING",
			opts:  "-s 10.0.3.0/24 ! -d 10.0.3.0/24 -j MASQUERADE",
		},
	}

	if !(reflect.DeepEqual(actual, exp)) {
		t.Errorf("Generated wrong routes.\nExpected:\n%+v\n\nGot:\n%+v\n",
			exp, actual)
	}
}

func TestMakeOFRule(t *testing.T) {
	flows := []string{
		"cookie=0x0, duration=997.526s, table=0, n_packets=0, " +
			"n_bytes=0, idle_age=997, priority=5000,in_port=3 actions=output:7",

		"cookie=0x0, duration=997.351s, table=1, n_packets=0, " +
			"n_bytes=0, idle_age=997, priority=4000,ip,dl_dst=0a:00:00:00:00:00," +
			"nw_dst=10.1.4.66 actions=LOCAL",

		"cookie=0x0, duration=159.314s, table=2, n_packets=0, n_bytes=0, " +
			"idle_age=159, priority=4000,ip,dl_dst=0a:00:00:00:00:00,nw_dst=10.1.4.66 " +
			"actions=mod_dl_dst:02:00:0a:00:96:72,resubmit(,2)",

		"cookie=0x0, duration=159.314s, table=2, n_packets=0, n_bytes=0, " +
			"idle_age=159, priority=5000,in_port=6 actions=resubmit(,1)," +
			"multipath(symmetric_l3l4,0,modulo_n,2,0,NXM_NX_REG0[0..1])",

		"table=2 priority=5000,in_port=6 actions=output:3",
	}

	var actual []OFRule
	for _, f := range flows {

		rule, err := makeOFRule(f)
		if err != nil {
			t.Errorf("failed to make OpenFlow rule: %s", err)
		}
		actual = append(actual, rule)
	}

	exp0 := OFRule{
		table:   "table=0",
		match:   "in_port=3,priority=5000",
		actions: "output:7",
	}

	exp1 := OFRule{
		table:   "table=1",
		match:   "dl_dst=0a:00:00:00:00:00,ip,nw_dst=10.1.4.66,priority=4000",
		actions: "LOCAL",
	}

	exp2 := OFRule{
		table:   "table=2",
		match:   "dl_dst=0a:00:00:00:00:00,ip,nw_dst=10.1.4.66,priority=4000",
		actions: "mod_dl_dst:02:00:0a:00:96:72,resubmit(,2)",
	}

	exp3 := OFRule{
		table:   "table=2",
		match:   "in_port=6,priority=5000",
		actions: "multipath(symmetric_l3l4,0,modulo_n,2,0,NXM_NX_REG0[0..1]),resubmit(,1)",
	}

	exp4 := OFRule{
		table:   "table=2",
		match:   "in_port=6,priority=5000",
		actions: "output:3",
	}

	exp := []OFRule{
		exp0,
		exp1,
		exp2,
		exp3,
		exp4,
	}

	if !(reflect.DeepEqual(actual, exp)) {
		t.Errorf("generated wrong OFRules.\nExpected:\n%+v\n\nGot:\n%+v\n",
			exp, actual)
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
