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
