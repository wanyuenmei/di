package dsl

import "fmt"

type funcImpl struct {
	do      func([]ast) (ast, error)
	minArgs int
}

var funcImplMap = map[astIdent]funcImpl{
	"+":    {arithFun(func(a, b int) int { return a + b }), 2},
	"-":    {arithFun(func(a, b int) int { return a - b }), 2},
	"*":    {arithFun(func(a, b int) int { return a * b }), 2},
	"/":    {arithFun(func(a, b int) int { return a / b }), 2},
	"%":    {arithFun(func(a, b int) int { return a % b }), 2},
	"list": {listImpl, 0},
}

func arithFun(do func(a, b int) int) func([]ast) (ast, error) {
	return func(args []ast) (ast, error) {
		var ints []int
		for _, arg := range args {
			ival, ok := arg.(astInt)
			if !ok {
				err := fmt.Errorf("bad arithmetic argument: %s", arg)
				return nil, err
			}
			ints = append(ints, int(ival))
		}

		total := ints[0]
		for _, x := range ints[1:] {
			total = do(total, x)
		}

		return astInt(total), nil
	}
}

func listImpl(args []ast) (ast, error) {
	return astList(args), nil
}
