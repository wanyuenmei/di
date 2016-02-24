// Package join implements a generic interface for matching elements from two slices
// similar in spirit to a database Join.
package join

import "reflect"

// A Pair represents an element from the left slice and an element from the right slice,
// that have been matched by a join.
type Pair struct {
	L, R interface{}
}

// Join attempts to match each element in `lSlice` with an element in `rSlice` in
// accordance with a score function.  If such a match is found, it is returned as an
// element of `pairs`, while leftover elements from `lSlice` and `rSlice` that couldn`t
// be matched, are returned as elements of `lonelyLefts` and `lonelyRights` respectively.
// Both `lSlice` and `rSlice` must be slice or array types, but they do not necessarily
// have to have the same type.
//
// Matches are made in accordance with the provided `score` function.  It takes a single
// element from `lSlice`, and a single element from `rSlice`, and computes a score
// suggesting their match preference.  The algorithm prefers to match pairs with the
// the score closest to zero (inclusive). Negative scores are never matched.
func Join(lSlice, rSlice interface{}, score func(left, right interface{}) int) (
	pairs []Pair, lonelyLefts, lonelyRights []interface{}) {

	val := reflect.ValueOf(rSlice)
	len := val.Len()
	lonelyRights = make([]interface{}, 0, len)

	for i := 0; i < len; i++ {
		lonelyRights = append(lonelyRights, val.Index(i).Interface())
	}

	val = reflect.ValueOf(lSlice)
	len = val.Len()

Outer:
	for i := 0; i < len; i++ {
		l := val.Index(i).Interface()
		bestScore := -1
		bestIndex := -1
		for i, r := range lonelyRights {
			s := score(l, r)
			switch {
			case s < 0:
				continue
			case s == 0:
				pairs = append(pairs, Pair{l, r})
				lonelyRights = sliceDel(lonelyRights, i)
				continue Outer
			case s < bestScore || bestScore < 0:
				bestIndex = i
				bestScore = s
			}
		}

		if bestIndex >= 0 {
			pairs = append(pairs, Pair{l, lonelyRights[bestIndex]})
			lonelyRights = sliceDel(lonelyRights, bestIndex)
			continue Outer
		}

		lonelyLefts = append(lonelyLefts, l)
	}

	return pairs, lonelyLefts, lonelyRights
}

func sliceDel(slice []interface{}, i int) []interface{} {
	l := len(slice)
	slice[i] = slice[l-1]
	slice[l-1] = nil // Allow garbage collection.
	return slice[:l-1]
}
