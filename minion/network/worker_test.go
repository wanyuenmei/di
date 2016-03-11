package network

import (
	"fmt"
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
