package main

import (
	"bufio"
	"testing"

	"github.com/NetSys/di/db"
	"github.com/NetSys/di/dsl"
	"github.com/NetSys/di/engine"
	"github.com/NetSys/di/util"
)

func configRunOnce(configPath string) error {
	f, err := util.Open(configPath)
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
	testConfig("specs/spark/spark.spec")
	testConfig("specs/zookeeper/zookeeper.spec")
}
