package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"os/exec"
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
	role := os.Getenv("ROLE")
	if len(role) == 0 {
		fmt.Println("redis require ROLE environment variable.")
		os.Exit(1)
	}

	auth := os.Getenv("AUTH")
	if len(auth) == 0 {
		fmt.Println("redis require AUTH environment variable.")
		os.Exit(1)
	}

	f, err := os.Create("/redis.conf")
	if err != nil {
		fmt.Println("error creating redis config file: ", err)
		os.Exit(1)
	}
	defer f.Close()

	var configs []string
	if role == "master" {
		configs = []string{
			"tcp-keepalive 60\n",
			fmt.Sprintf("requirepass %s\n", auth),
		}
	} else if role == "worker" {
		master := os.Getenv("MASTER")
		if len(master) == 0 {
			fmt.Println("redis require MASTER environment variable.")
			os.Exit(1)
		}

		resolveHostname(master)

		configs = []string{
			fmt.Sprintf("masterauth %s\n", auth),
			fmt.Sprintf("slaveof %s 6379\n", master),
		}
	} else {
		fmt.Println("redis require either master or worker ROLE.")
		os.Exit(1)
	}

	w := bufio.NewWriter(f)
	for _, l := range configs {
		if _, err := w.WriteString(l); err != nil {
			fmt.Println("error generating redis config file: ", err)
			os.Exit(1)
		}
	}

	w.Flush()

	cmd := exec.Command("redis-server", "/redis.conf")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()
}
