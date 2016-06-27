package ovsdb

import "testing"

func TestNewCondition(t *testing.T) {
	cond := newCondition("col", "func", "test")
	if len(cond.([]interface{})) != 3 {
		t.Error("Condition should have length 3.")
	}
}
