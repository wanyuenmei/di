package specs

import (
	"bufio"
	"os"
	"path/filepath"
	"testing"
	"text/scanner"

	"github.com/NetSys/quilt/stitch"
	"github.com/NetSys/quilt/util"
)

func configRunOnce(configPath string, quiltPath string) error {
	f, err := util.Open(configPath)
	if err != nil {
		return err
	}
	defer f.Close()

	var sc scanner.Scanner
	_, err = stitch.New(*sc.Init(bufio.NewReader(f)), quiltPath, false)
	if err != nil {
		return err
	}

	return nil
}

func TestConfigs(t *testing.T) {
	testConfig := func(configPath string, quiltPath string) {
		if err := configRunOnce(configPath, quiltPath); err != nil {
			t.Errorf("%s failed validation: %s \n quiltPath: %s",
				configPath, err.Error(), quiltPath)
		}
	}

	goPath := os.Getenv("GOPATH")
	quiltPath := filepath.Join(goPath, "src")

	testConfig("example.spec", "specs/stdlib")
	testConfig("../quilt-tester/config/config.spec", "specs/stdlib")
	testConfig("./spark/sparkPI.spec", quiltPath)
	testConfig("./wordpress/main.spec", quiltPath)
	testConfig("./etcd/example.spec", quiltPath)
	testConfig("./redis/example.spec", quiltPath)
}
