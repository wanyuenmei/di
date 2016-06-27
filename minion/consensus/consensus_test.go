package consensus

import "testing"

func TestCreate(t *testing.T) {
	if m := NewMock(); m == nil {
		t.Error("Failed to create mock Store.")
	}
}
