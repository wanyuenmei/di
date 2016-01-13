package consensus

import (
	"fmt"
	"net"
	"reflect"
	"testing"

	"github.com/NetSys/di/db"
	"github.com/davecgh/go-spew/spew"
)

func TestLabelDiff(t *testing.T) {
	check := func(kvLabels map[string]*labelData, containers []db.Container,
		expRemove []string, expChange map[string]*labelData) string {
		remove, change := diffLabels(kvLabels, containers)
		if !reflect.DeepEqual(remove, expRemove) {
			return spew.Sprintf("\nRemove %s\nExpected %s", remove,
				expRemove)
		}

		for label, ld := range expChange {
			if ld.IP != "" {
				continue
			}

			c, ok := change[label]
			if !ok {
				return fmt.Sprintf("Missing label: %s")
			}

			ld.IP = c.IP
		}

		for _, ld := range change {
			ip := net.ParseIP(ld.IP).To4()
			if ip == nil || ip[0] != 10 || ip[1] != 1 {
				return fmt.Sprintf("\nInvalid IP: %s\nIn: %s", ld.IP,
					change)
			}
		}

		if !reflect.DeepEqual(change, expChange) {
			return spew.Sprintf("\nChange %s\nExpected %s", change,
				expChange)
		}

		return ""
	}

	kvLabels := map[string]*labelData{}
	containers := []db.Container{}

	var expRemove []string
	expChange := map[string]*labelData{}
	if res := check(kvLabels, containers, expRemove, expChange); res != "" {
		t.Error(res)
	}

	containers = []db.Container{
		{SchedID: "1", Labels: []string{"a"}},
		{SchedID: "2", Labels: []string{"a", "b"}},
		{SchedID: "3", Labels: []string{"a", "b", "c"}},
		{SchedID: "4", Labels: []string{"a", "b", "c", "d"}},
	}
	expChange = map[string]*labelData{
		"a": {"", []string{"1", "2", "3", "4"}},
		"b": {"", []string{"2", "3", "4"}},
		"c": {"", []string{"3", "4"}},
		"d": {"", []string{"4"}},
	}
	if res := check(kvLabels, containers, expRemove, expChange); res != "" {
		t.Error(res)
	}

	kvLabels = map[string]*labelData{
		"a": {"10.1.0.1", []string{"1", "2", "3", "4"}},
		"b": {"10.1.0.2", []string{"2", "3", "4"}},
		"c": {"10.1.0.3", []string{"3", "4"}},
		"d": {"10.1.0.4", []string{"4"}},
	}
	containers = []db.Container{
		{SchedID: "1", Labels: []string{"a"}},
		{SchedID: "2", Labels: []string{"a", "b"}},
		{SchedID: "3", Labels: []string{"a", "b", "c"}},
		{SchedID: "4", Labels: []string{"a", "b", "c", "d"}},
	}
	expChange = map[string]*labelData{}
	if res := check(kvLabels, containers, expRemove, expChange); res != "" {
		t.Error(res)
	}

	containers = []db.Container{
		{SchedID: "1", Labels: []string{"a"}},
		{SchedID: "2", Labels: []string{"a", "b"}},
		{SchedID: "4", Labels: []string{"a", "b", "c"}},
	}
	expChange = map[string]*labelData{
		"a": {"10.1.0.1", []string{"1", "2", "4"}},
		"b": {"10.1.0.2", []string{"2", "4"}},
		"c": {"10.1.0.3", []string{"4"}},
	}
	expRemove = []string{"d"}
	if res := check(kvLabels, containers, expRemove, expChange); res != "" {
		t.Error(res)
	}

	kvLabels = map[string]*labelData{
		"a": {"Broken", []string{"1"}},
		"b": {"11.1.0.2", []string{"2"}},
	}
	containers = []db.Container{
		{SchedID: "1", Labels: []string{"a"}},
		{SchedID: "2", Labels: []string{"b"}},
	}
	expChange = map[string]*labelData{
		"a": {"", []string{"1"}},
		"b": {"", []string{"2"}},
	}
	expRemove = nil
	if res := check(kvLabels, containers, expRemove, expChange); res != "" {
		t.Error(res)
	}

}
