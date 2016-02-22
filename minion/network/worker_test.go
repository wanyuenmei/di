package network

import (
	"fmt"
	"github.com/NetSys/di/db"
	"testing"
)

func TestNoConnections(t *testing.T) {
	labels, connections := defaultLabelsConnections()
	dbc := db.Container{
		ID:     1,
		Labels: []string{"green"},
	}

	exp := fmt.Sprintf("127.0.0.1\tlocalhost\n")
	actual := generateEtcHosts(dbc, labels, connections)

	if exp != actual {
		t.Error("Generated wrong basic /etc/hosts.")
	}
}

func TestImplementsSingleLabel(t *testing.T) {
	labels, connections := defaultLabelsConnections()
	dbc := db.Container{
		ID:     2,
		Labels: []string{"red"},
	}

	exp := fmt.Sprintf("10.0.0.2\tblue.di\n10.0.0.3\tgreen.di\n127.0.0.1\tlocalhost\n")
	actual := generateEtcHosts(dbc, labels, connections)

	if exp != actual {
		t.Error("Generated wrong single label /etc/hosts.")
	}
}

func TestImplementsMultipleLabels(t *testing.T) {
	labels, connections := defaultLabelsConnections()
	dbc := db.Container{
		ID:     3,
		Labels: []string{"red", "blue"},
	}

	exp := fmt.Sprintf("10.0.0.1\tred.di\n10.0.0.2\tblue.di\n10.0.0.3\tgreen.di\n127.0.0.1\tlocalhost\n")
	actual := generateEtcHosts(dbc, labels, connections)

	if exp != actual {
		t.Error("Generated wrong multi-label /etc/hosts")
	}
}

// Both red and blue connect to green. Make sure that green.di only appears once in /etc/hosts.
func TestDuplicateConnections(t *testing.T) {
	labels, connections := defaultLabelsConnections()
	dbc := db.Container{
		ID:     4,
		Labels: []string{"red", "blue"},
	}

	connections["blue"] = append(connections["blue"], "green")

	exp := fmt.Sprintf("10.0.0.1\tred.di\n10.0.0.2\tblue.di\n10.0.0.3\tgreen.di\n127.0.0.1\tlocalhost\n")
	actual := generateEtcHosts(dbc, labels, connections)

	if exp != actual {
		t.Error("Generated wrong /etc/hosts for duplicate connections.")
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
