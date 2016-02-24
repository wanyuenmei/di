package join

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/davecgh/go-spew/spew"
)

func TestJoin(t *testing.T) {
	score := func(left, right interface{}) int {
		return left.(int) - right.(int)
	}

	pairs, left, right := Join([]int{10, 11, 12}, []int{10, 11, 12}, score)
	if len(left) > 0 {
		t.Errorf("Unexpected lefts: %s", left)
	}
	if len(right) > 0 {
		t.Errorf("Unexpected rights: %s", right)
	}
	if !eq(pairs, []Pair{{10, 10}, {11, 11}, {12, 12}}) {
		t.Error(spew.Sprintf("Unexpected pairs: %s", pairs))
	}

	pairs, left, right = Join([]int{10, 11, 12}, []int{13, 1, 2}, score)
	if !eq(left, []interface{}{12}) {
		t.Error(spew.Sprintf("Unexpected left: %s", left))
	}
	if !eq(right, []interface{}{13}) {
		t.Error(spew.Sprintf("Unexpected right %s", right))
	}
	if !eq(pairs, []Pair{{10, 2}, {11, 1}}) {
		t.Error(spew.Sprintf("Unexpected pairs: %s", pairs))
	}
}

func ExampleJoin() {
	lefts := []string{"a", "bc", "def"}
	rights := []int{0, 2, 4}
	score := func(left, right interface{}) int {
		return len(left.(string)) - right.(int)
	}
	pairs, lonelyLefts, lonelyRights := Join(lefts, rights, score)

	fmt.Println(pairs, lonelyLefts, lonelyRights)
	// Output: [{a 0} {bc 2}] [def] [4]
}

func eq(a, b interface{}) bool {
	return reflect.DeepEqual(a, b)
}
