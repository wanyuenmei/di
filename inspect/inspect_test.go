package inspect

import (
	"strings"
	"testing"
	"text/scanner"

	"github.com/NetSys/quilt/stitch"
)

func TestSlug(t *testing.T) {
	test := map[string]string{
		"slug.spec":       "slug",
		"a/b/c/slug.spec": "a/b/c/slug",
		"foo":             "err",
	}

	for inp, expect := range test {
		if sl, err := getSlug(inp); err != nil {
			if expect != "err" {
				t.Error(err)
			}
		} else if sl != expect {
			t.Error(sl)
		}
	}
}

func initSpec(src string) (stitch.Stitch, error) {
	var sc scanner.Scanner
	spec, err := stitch.New(*sc.Init(strings.NewReader(src)), "../specs", false)
	return spec, err
}

func TestViz(t *testing.T) {
	expect := `strict digraph {
    subgraph cluster_0 {
        1; 2; 3; public;
    }
    1 -> 2
    2 -> 3
}`
	stc := `(label "a" (docker "ubuntu"))
(label "b" (docker "ubuntu"))
(label "c" (docker "ubuntu"))

(connect 22 "a" "b")
(connect 22 "b" "c")`

	spec, err := initSpec(stc)
	if err != nil {
		panic(err)
	}

	graph, err := stitch.InitializeGraph(spec)
	if err != nil {
		panic(err)
	}

	gv := makeGraphviz(graph)
	gv = strings.Replace(gv, "\n", "", -1)
	gv = strings.Replace(gv, " ", "", -1)
	expect = strings.Replace(expect, "\n", "", -1)
	expect = strings.Replace(expect, " ", "", -1)
	if gv != expect {
		t.Error(gv + "\n" + expect)
	}
}
