package stitch

import (
	"strings"
	"testing"
	"text/scanner"
)

func initSpec(src string) (Stitch, error) {
	var sc scanner.Scanner
	spec, err := New(*sc.Init(strings.NewReader(src)), "../specs", false)
	return spec, err
}

func TestReach(t *testing.T) {
	stc := `(label "a" (docker "ubuntu"))
(label "b" (docker "ubuntu"))
(label "c" (docker "ubuntu"))

(connect 22 "a" "b")
(connect 22 "b" "c")

(invariant reach true "a" "c")
(invariant reach false "c" "a")
(invariant between true "a" "c" "b")
(invariant between false "c" "a" "b")`
	_, err := initSpec(stc)
	if err != nil {
		t.Error(err)
	}
}

func TestFail(t *testing.T) {
	stc := `(label "a" (docker "ubuntu"))
(label "b" (docker "ubuntu"))
(label "c" (docker "ubuntu"))

(connect 22 "a" "b")
(connect 22 "b" "c")

(invariant reach true "a" "c")
(invariant reach true "c" "a")`
	expectedFailure := `invariant failed: reach true "c" "a"`
	if _, err := initSpec(stc); err == nil {
		t.Errorf("got no error, expected %s", expectedFailure)
	} else if err.Error() != expectedFailure {
		t.Errorf("got error %s, expected %s", err, expectedFailure)
	}
}

func TestBetween(t *testing.T) {
	stc := `(label "a" (docker "ubuntu"))
(label "b" (docker "ubuntu"))
(label "c" (docker "ubuntu"))
(label "d" (docker "ubuntu"))
(label "e" (docker "ubuntu"))

(connect 22 "a" "b")
(connect 22 "a" "c")
(connect 22 "b" "d")
(connect 22 "c" "d")
(connect 22 "d" "e")

(invariant reach true "a" "e")
(invariant between true "a" "e" "d")`
	_, err := initSpec(stc)
	if err != nil {
		t.Error(err)
	}
}

func TestNoConnect(t *testing.T) {
	t.Skip("wait for scheduler, use the new scheduling algorithm")
	stc := `(label "a" (docker "ubuntu"))
(label "b" (docker "ubuntu"))
(label "c" (docker "ubuntu"))
(label "d" (docker "ubuntu"))
(label "e" (docker "ubuntu"))

(let ((cfg (list (provider "Amazon")
                 (region "us-west-1")
                 (size "m4.2xlarge")
                 (diskSize 32))))
    (makeList 4 (machine (role "test") cfg)))

(place (labelRule "exclusive" "e") "b" "d")
(place (labelRule "exclusive" "c") "b" "d" "e")
(place (labelRule "exclusive" "a") "c" "d" "e")

(invariant enough)`
	_, err := initSpec(stc)
	if err != nil {
		t.Error(err)
	}
}

func TestNested(t *testing.T) {
	t.Skip("needs hierarchical labeling to pass")
	stc := `(label "a" (docker "ubuntu"))
(label "b" (docker "ubuntu"))
(label "c" (docker "ubuntu"))
(label "d" (docker "ubuntu"))

(label "g1" "a" "b")
(label "g2" "c" "d")

(connect 22 "g1" "g2")

(invariant reach true "a" "d")
(invariant reach true "b" "c")`
	_, err := initSpec(stc)
	if err != nil {
		t.Error(err)
	}
}

func TestPlacementInvs(t *testing.T) {
	t.Skip("wait for scheduler, use the new scheduling algorithm")
	stc := `(label "a" (docker "ubuntu"))
(label "b" (docker "ubuntu"))
(label "c" (docker "ubuntu"))
(label "d" (docker "ubuntu"))
(label "e" (docker "ubuntu"))

(connect 22 "a" "b")
(connect 22 "a" "c")
(connect 22 "b" "d")
(connect 22 "c" "d")
(connect 22 "d" "e")
(connect 22 "c" "e")

(let ((cfg (list (provider "Amazon")
                 (region "us-west-1")
                 (size "m4.2xlarge")
                 (diskSize 32))))
    (makeList 4 (machine (role "test") cfg)))

(place (labelRule "exclusive" "e") "b" "d")
(place (labelRule "exclusive" "c") "b" "d" "e")
(place (labelRule "exclusive" "a") "c" "d" "e")

(invariant reach true "a" "e")
(invariant enough)`
	_, err := initSpec(stc)
	if err != nil {
		t.Error(err)
	}
}
