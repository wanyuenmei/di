package util

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
)

func httpRequest(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "<error>", err
	}

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "<error>", err
	}

	return strings.TrimSpace(string(body)), err
}

func MyIp() (string, error) {
	return httpRequest("http://checkip.amazonaws.com/")
}

func NewDiscoveryToken(memberCount int) (string, error) {
	return httpRequest(fmt.Sprintf("https://discovery.etcd.io/new?size=%d",
		memberCount))
}

func CloudConfig(keys []string) string {
	cloudConfig := `#cloud-config

coreos:
    units:
        - name: minion.service
          command: start
          content: |
            [Unit]
            Description=DI Minion
            After=docker.service
            Requires=docker.service

            [Service]
            ExecStartPre=-/usr/bin/docker kill minion
            ExecStartPre=-/usr/bin/docker rm minion
            ExecStartPre=/usr/bin/docker pull quay.io/netsys/di-minion
            ExecStart=/usr/bin/docker run --net=host --name=minion --privileged \
            -v /var/run/docker.sock:/var/run/docker.sock quay.io/netsys/di-minion

`

	if len(keys) > 0 {
		cloudConfig += "ssh_authorized_keys:\n"
		for _, key := range keys {
			cloudConfig += fmt.Sprintf("    - \"%s\"\n", key)
		}
	}

	return cloudConfig
}
