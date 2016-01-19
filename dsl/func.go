package dsl

import "fmt"

type funcImpl struct {
	do      func(*evalCtx, []ast) (ast, error)
	minArgs int
}

var funcImplMap = map[astIdent]funcImpl{
	"%":        {arithFun(func(a, b int) int { return a % b }), 2},
	"*":        {arithFun(func(a, b int) int { return a * b }), 2},
	"+":        {arithFun(func(a, b int) int { return a + b }), 2},
	"-":        {arithFun(func(a, b int) int { return a - b }), 2},
	"/":        {arithFun(func(a, b int) int { return a / b }), 2},
	"connect":  {connectImpl, 3},
	"label":    {labelImpl, 2},
	"list":     {listImpl, 0},
	"makeList": {makeListImpl, 2},
	"sprintf":  {sprintfImpl, 1},
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

func connectImpl(ctx *evalCtx, args__ []ast) (ast, error) {
	args, err := evalArgs(ctx, args__)
	if err != nil {
		return nil, err
	}

	var min, max int
	switch t := args[0].(type) {
	case astInt:
		min, max = int(t), int(t)
	case astList:
		if len(t) != 2 {
			return nil, fmt.Errorf("port range must have two ints: %s", t)
		}

		minAst, minOK := t[0].(astInt)
		maxAst, maxOK := t[1].(astInt)
		if !minOK || !maxOK {
			return nil, fmt.Errorf("port range must have two ints: %s", t)
		}

		min, max = int(minAst), int(maxAst)
	default:
		return nil, fmt.Errorf("port range must be an int or a list of ints:"+
			" %s", args[0])
	}

	if min < 0 || max > 65535 {
		return nil, fmt.Errorf("invalid port range: [%d, %d]", min, max)
	}

	if min > max {
		return nil, fmt.Errorf("invalid port range: [%d, %d]", min, max)
	}

	var labels []string
	for _, arg := range flatten(args[1:]) {
		label, ok := arg.(astString)
		if !ok {
			err := fmt.Errorf("connect applies to labels: %s", arg)
			return nil, err
		}

		if _, ok := ctx.labels[string(label)]; !ok {
			return nil, fmt.Errorf("connect undefined label: %s",
				label)
		}

		labels = append(labels, string(label))
	}

	from := labels[0]
	for _, to := range labels[1:] {
		cn := Connection{
			From:    from,
			To:      to,
			MinPort: min,
			MaxPort: max,
		}
		ctx.connections[cn] = struct{}{}
	}

	newArgs := args[0:1]
	for _, label := range labels {
		newArgs = append(newArgs, astString(label))
	}

	return astFunc{astIdent("connect"), connectImpl, newArgs}, nil
}

func labelImpl(ctx *evalCtx, args__ []ast) (ast, error) {
	args, err := evalArgs(ctx, args__)
	if err != nil {
		return nil, err
	}

	str, ok := args[0].(astString)
	if !ok {
		return nil, fmt.Errorf("label must be a string, found: %s", args[0])
	}
	label := string(str)

	if _, ok := ctx.labels[label]; ok {
		return nil, fmt.Errorf("attempt to redefine label: %s", label)
	}

	var containers []*Container
	for _, elem := range flatten(args[1:]) {
		switch t := elem.(type) {
		case astAtom:
			containers = append(containers, ctx.containers[t.index])
		case astString:
			children, ok := ctx.labels[string(t)]
			if !ok {
				return nil, fmt.Errorf("undefined label: %s", t)
			}

			for _, c := range children {
				containers = append(containers, c)
			}
		default:
			return nil, fmt.Errorf("label must apply to atoms or other"+
				" labels, found: %s", elem)
		}
	}

	for _, c := range containers {
		if len(c.Labels) > 0 && c.Labels[len(c.Labels)-1] == label {
			// It's possible that the same container appears in the list
			// twice.  If that's the case, we'll end up labelling it multiple
			// times unless we check it's most recently added label.
			continue
		}

		c.Labels = append(c.Labels, label)

	}

	ctx.labels[label] = containers
	return astFunc{astIdent("label"), labelImpl, args}, nil
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

func sprintfImpl(ctx *evalCtx, args__ []ast) (ast, error) {
	args, err := evalArgs(ctx, args__)
	if err != nil {
		return nil, err
	}

	format, ok := args[0].(astString)
	if !ok {
		return nil, fmt.Errorf("sprintf format must be a string: %s", args[0])
	}

	var ifaceArgs []interface{}
	for _, arg := range args[1:] {
		var iface interface{}
		switch t := arg.(type) {
		case astString:
			iface = string(t)
		case astInt:
			iface = int(t)
		default:
			iface = t
		}

		ifaceArgs = append(ifaceArgs, iface)
	}

	return astString(fmt.Sprintf(string(format), ifaceArgs...)), nil
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

func flatten(lst []ast) []ast {
	var result []ast

	for _, l := range lst {
		switch t := l.(type) {
		case astList:
			result = append(result, flatten(t)...)
		default:
			result = append(result, l)
		}
	}

	return result
}
