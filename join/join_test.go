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

type JoinList []interface{}

func (jil JoinList) Len() int {
	return len(jil)
}

func (jil JoinList) Get(ii int) interface{} {
	return jil[ii]
}

type JoinInt int

func (ji JoinInt) JoinKey() interface{} {
	return ji
}

func TestHashJoin(t *testing.T) {
	keyFunc := func(val interface{}) interface{} {
		return val
	}
	pairs, left, right := HashJoin(JoinList{10, 11, 12},
		JoinList{10, 11, 12}, keyFunc, keyFunc)
	if len(left) > 0 {
		t.Errorf("Unexpected lefts: %s", left)
	}
	if len(right) > 0 {
		t.Errorf("Unexpected rights: %s", right)
	}
	if !eq(pairs, []Pair{{10, 10}, {11, 11}, {12, 12}}) {
		t.Error(spew.Sprintf("Unexpected pairs: %s", pairs))
	}

	pairs, left, right = HashJoin(JoinList{10, 11, 12},
		JoinList{13, 11, 2}, keyFunc, keyFunc)
	if len(left) != 2 {
		t.Error(spew.Sprintf("Unexpected left: %s", left))
	}
	if len(right) != 2 {
		t.Error(spew.Sprintf("Unexpected right %s", right))
	}
	if !eq(pairs, []Pair{{11, 11}}) {
		t.Error(spew.Sprintf("Unexpected pairs: %s", pairs))
	}
}

func TestHashJoinNilKeyFunc(t *testing.T) {
	keyFunc := func(val interface{}) interface{} {
		return val
	}
	pairs, left, right := HashJoin(JoinList{10, 11, 12},
		JoinList{10, 11, 12}, nil, keyFunc)
	if len(left) > 0 {
		t.Errorf("Unexpected lefts: %s", left)
	}
	if len(right) > 0 {
		t.Errorf("Unexpected rights: %s", right)
	}
	if !eq(pairs, []Pair{{10, 10}, {11, 11}, {12, 12}}) {
		t.Error(spew.Sprintf("Unexpected pairs: %s", pairs))
	}

	pairs, left, right = HashJoin(JoinList{10, 11, 12},
		JoinList{13, 11, 2}, keyFunc, nil)
	if len(left) != 2 {
		t.Error(spew.Sprintf("Unexpected left: %s", left))
	}
	if len(right) != 2 {
		t.Error(spew.Sprintf("Unexpected right %s", right))
	}
	if !eq(pairs, []Pair{{11, 11}}) {
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
