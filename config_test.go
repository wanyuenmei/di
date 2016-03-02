package main

import (
	"bufio"
	"os"
	"testing"

	"github.com/NetSys/di/db"
	"github.com/NetSys/di/dsl"
	"github.com/NetSys/di/engine"
)

func configRunOnce(configPath string) error {
	f, err := os.Open(configPath)
	if err != nil {
		return err
	}
	defer f.Close()

	spec, err := dsl.New(bufio.NewReader(f))
	if err != nil {
		return err
	}

	err = engine.UpdatePolicy(db.New(), spec)
	if err != nil {
		return err
	}

	return nil
}

func TestConfigs(t *testing.T) {
	testConfig := func(configPath string) {
		if err := configRunOnce(configPath); err != nil {
			t.Errorf("%s failed validation: %s", configPath, err.Error())
		}
	}
	testConfig("./config.spec")
	testConfig("di-tester/config/config.spec")
}
