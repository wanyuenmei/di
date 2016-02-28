package util

import (
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

// MyIP gets the local systems Public IP address as visible on the WAN by querying an
// exeternal service.
func MyIP() (string, error) {
	return httpRequest("http://checkip.amazonaws.com/")
}
