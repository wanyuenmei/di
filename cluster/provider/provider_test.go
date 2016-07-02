package provider

import (
	"testing"

	"github.com/NetSys/quilt/db"
	"github.com/NetSys/quilt/stitch"
)

func TestDefaultRegion(t *testing.T) {
	exp := "foo"
	m := db.Machine{Provider: "Amazon", Region: exp}
	m = DefaultRegion(m)
	if m.Region != exp {
		t.Errorf("expected %s, found %s", exp, m.Region)
	}

	m.Region = ""
	m = DefaultRegion(m)
	exp = "us-west-1"
	if m.Region != exp {
		t.Errorf("expected %s, found %s", exp, m.Region)
	}

	m.Region = ""
	m.Provider = "Google"
	exp = "us-east1-b"
	m = DefaultRegion(m)
	if m.Region != exp {
		t.Errorf("expected %s, found %s", exp, m.Region)
	}

	m.Region = ""
	m.Provider = "Azure"
	exp = "Central US"
	m = DefaultRegion(m)
	if m.Region != exp {
		t.Errorf("expected %s, found %s", exp, m.Region)
	}

	m.Region = ""
	m.Provider = "Vagrant"
	exp = ""
	m = DefaultRegion(m)
	if m.Region != exp {
		t.Errorf("expected %s, found %s", exp, m.Region)
	}

	paniced := false
	m.Region = ""
	m.Provider = "Panic"
	exp = ""
	func() {
		defer func() {
			r := recover()
			paniced = r != nil
		}()

		m = DefaultRegion(m)
	}()

	if !paniced {
		t.Error("Expected panic")
	}
}

func TestConstraints(t *testing.T) {
	checkConstraint := func(descriptions []Description, ram stitch.Range,
		cpu stitch.Range, maxPrice float64, exp string) {
		resSize := pickBestSize(descriptions, ram, cpu, maxPrice)
		if resSize != exp {
			t.Errorf("bad size picked. Expected %s, got %s", exp, resSize)
		}
	}

	// Test all constraints specified with valid price
	testDescriptions := []Description{
		{Size: "size1", Price: 2, RAM: 2, CPU: 2},
	}
	checkConstraint(testDescriptions, stitch.Range{Min: 1, Max: 3},
		stitch.Range{Min: 1, Max: 3}, 2, "size1")

	// Test no max
	checkConstraint(testDescriptions, stitch.Range{Min: 1},
		stitch.Range{Min: 1}, 2, "size1")

	// Test exact match
	checkConstraint(testDescriptions, stitch.Range{Min: 2},
		stitch.Range{Min: 2}, 2, "size1")

	// Test no match
	checkConstraint(testDescriptions, stitch.Range{Min: 3},
		stitch.Range{Min: 2}, 2, "")

	// Test price too expensive
	checkConstraint(testDescriptions, stitch.Range{Min: 2},
		stitch.Range{Min: 2}, 1, "")

	// Test multiple matches (should pick cheapest)
	testDescriptions = []Description{
		{Size: "size2", Price: 2, RAM: 8, CPU: 4},
		{Size: "size3", Price: 1, RAM: 4, CPU: 4},
		{Size: "size4", Price: 0.5, RAM: 3, CPU: 4},
	}
	checkConstraint(testDescriptions, stitch.Range{Min: 4},
		stitch.Range{Min: 3}, 2, "size3")

	// Test infinite price
	checkConstraint(testDescriptions, stitch.Range{Min: 4},
		stitch.Range{Min: 3}, 0, "size3")

	// Test default ranges (should pick cheapest)
	checkConstraint(testDescriptions, stitch.Range{},
		stitch.Range{}, 0, "size4")

	// Test one default range (should pick only on the specified range)
	checkConstraint(testDescriptions, stitch.Range{Min: 4},
		stitch.Range{}, 0, "size3")
	checkConstraint(testDescriptions, stitch.Range{Min: 3},
		stitch.Range{}, 0, "size4")
}
