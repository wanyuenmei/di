package docker

import "testing"

func TestCreateDocker(t *testing.T) {
	if d := New("sock"); d == nil {
		t.Error("Failed to create a docker instance")
	}
}
