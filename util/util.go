package util

import (
	"archive/tar"
	"bytes"
	"io"
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

// ToTar returns a tar archive named NAME and containing CONTENT.
func ToTar(name string, permissions int, content string) (io.Reader, error) {
	buf := new(bytes.Buffer)
	tw := tar.NewWriter(buf)
	hdr := &tar.Header{
		Name: name,
		Mode: int64(permissions),
		Size: int64(len(content)),
	}

	if err := tw.WriteHeader(hdr); err != nil {
		return nil, err
	}

	if _, err := tw.Write([]byte(content)); err != nil {
		return nil, err
	}

	if err := tw.Close(); err != nil {
		return nil, err
	}

	return buf, nil
}

// MyIP gets the local systems Public IP address as visible on the WAN by querying an
// exeternal service.
func MyIP() (string, error) {
	return httpRequest("http://checkip.amazonaws.com/")
}
