package stitch

import (
	"bufio"
	"os"
)

func contains(ls []string, it string) bool {
	for _, a := range ls {
		if a == it {
			return true
		}
	}

	return false
}

type parser func(string) error

func forLineInFile(path string, f parser) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if err := f(line); err != nil {
			return err
		}
	}
	return nil
}

// Enumerate the cartesian product of the given flattened labels.
func enumerate(inp [][]interface{}) [][]interface{} {
	var combos [][]interface{}
	var enum func(idxs []int)
	enum = func(idxs []int) {
		if progress := len(idxs); progress == len(inp) {
			var argSet []interface{}
			for i, idx := range idxs {
				argSet = append(argSet, inp[i][idx])
			}

			combos = append(combos, argSet)
		} else {
			for i := range inp[progress] {
				enum(append(idxs, i))
			}
		}
	}
	enum([]int{})
	return combos
}
