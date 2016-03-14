package util

import (
	"archive/tar"
	"bytes"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"github.com/spf13/afero"
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

func ShortUUID(uuid string) string {
	if len(uuid) < 12 {
		return uuid
	}
	return uuid[:12]
}

// Stored in a variable so that we can replace it with in-memory filesystems for unit tests.
var AppFs afero.Fs = afero.NewOsFs()

func Open(path string) (afero.File, error) {
	return AppFs.Open(path)
}

func WriteFile(filename string, data []byte, perm os.FileMode) error {
	a := afero.Afero{
		Fs: AppFs,
	}
	return a.WriteFile(filename, data, perm)
}
