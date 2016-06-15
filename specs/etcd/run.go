package main

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"
)

func resolveHostname(hostname string) {
	for {
		if _, err := net.LookupHost(hostname); err == nil {
			return
		}
		time.Sleep(time.Second)
	}
}

func main() {
	peers := strings.Split(os.Getenv("PEERS"), ",")
	if len(peers) == 0 {
		fmt.Println("etcd require PEERS environment variable.")
		os.Exit(1)
	}

	host := os.Getenv("HOST")
	if len(host) == 0 {
		fmt.Println("etcd require HOST environment variable.")
		os.Exit(1)
	}

	initialCluster := []string{}
	for _, peer := range peers {
		resolveHostname(peer)
		node := fmt.Sprintf("%s=http://%s:2380", peer, peer)
		initialCluster = append(initialCluster, node)
	}
	initialClusterStr := strings.Join(initialCluster, ",")

	os.Setenv("ETCD_NAME", host)
	os.Setenv("ETCD_LISTEN_PEER_URLS", fmt.Sprintf("http://%s:2380", host))
	os.Setenv("ETCD_LISTEN_CLIENT_URLS", "http://0.0.0.0:2379")
	os.Setenv("ETCD_INITIAL_ADVERTISE_PEER_URLS", fmt.Sprintf("http://%s:2380", host))
	os.Setenv("ETCD_INITIAL_CLUSTER", initialClusterStr)
	os.Setenv("ETCD_INITIAL_CLUSTER_STATE", "new")
	os.Setenv("ETCD_ADVERTISE_CLIENT_URLS", fmt.Sprintf("http://%s:2379", host))

	cmd := exec.Command("./etcd")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()
}
