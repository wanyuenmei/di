package dsl

import "fmt"

type funcImpl struct {
	do      func(*evalCtx, []ast) (ast, error)
	minArgs int
}

var funcImplMap = map[astIdent]funcImpl{
	"+":        {arithFun(func(a, b int) int { return a + b }), 2},
	"-":        {arithFun(func(a, b int) int { return a - b }), 2},
	"*":        {arithFun(func(a, b int) int { return a * b }), 2},
	"/":        {arithFun(func(a, b int) int { return a / b }), 2},
	"%":        {arithFun(func(a, b int) int { return a % b }), 2},
	"list":     {listImpl, 0},
	"makeList": {makeListImpl, 2},
}

func arithFun(do func(a, b int) int) func(*evalCtx, []ast) (ast, error) {
	return func(ctx *evalCtx, args__ []ast) (ast, error) {
		args, err := evalArgs(ctx, args__)
		if err != nil {
			return nil, err
		}

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

func listImpl(ctx *evalCtx, args__ []ast) (ast, error) {
	args, err := evalArgs(ctx, args__)
	return astList(args), err
}

func makeListImpl(ctx *evalCtx, args []ast) (ast, error) {
	eval, err := args[0].eval(ctx)
	if err != nil {
		return nil, err
	}

	count, ok := eval.(astInt)
	if !ok || count < 0 {
		return nil, fmt.Errorf("makeList must begin with a positive integer, "+
			"found: %s", args[0])
	}

	var result []ast
	for i := 0; i < int(count); i++ {
		eval, err := args[1].eval(ctx)
		if err != nil {
			return nil, err
		}
		result = append(result, eval)
	}
	return astList(result), nil
}

func evalArgs(ctx *evalCtx, args []ast) ([]ast, error) {
	var result []ast
	for _, a := range args {
		eval, err := a.eval(ctx)
		if err != nil {
			return nil, err
		}
		result = append(result, eval)
	}

	return result, nil
}
