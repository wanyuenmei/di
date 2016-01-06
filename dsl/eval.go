package dsl

import "fmt"

type evalCtx struct {
	binds   map[astIdent]ast
	defines map[astIdent]ast
}

func eval(parsed ast) (ast, evalCtx, error) {
	ctx := evalCtx{make(map[astIdent]ast), make(map[astIdent]ast)}
	evaluated, err := parsed.eval(ctx)
	if err != nil {
		return nil, evalCtx{}, err
	}

	return evaluated, ctx, nil
}

func (root astRoot) eval(ctx evalCtx) (ast, error) {
	results, err := astList(root).eval(ctx)
	if err != nil {
		return nil, err
	}
	return astRoot(results.(astList)), nil
}

func (list astList) eval(ctx evalCtx) (ast, error) {
	result := []ast{}
	for _, elem := range list {
		evaled, err := elem.eval(ctx)
		if err != nil {
			return nil, err
		}
		result = append(result, evaled)
	}

	return astList(result), nil
}

func (fn astFunc) eval(ctx evalCtx) (ast, error) {
	var args []ast

	for _, arg := range fn.args {
		eval, err := arg.eval(ctx)
		if err != nil {
			return nil, err
		}
		args = append(args, eval)
	}

	return fn.do(args)
}

func (def astDefine) eval(ctx evalCtx) (ast, error) {
	if _, ok := ctx.defines[def.name]; ok {
		return nil, fmt.Errorf("attempt to redefine: \"%s\"", def.name)
	}

	result, err := def.ast.eval(ctx)
	if err != nil {
		return nil, err
	}

	ctx.defines[def.name] = result
	ctx.binds[def.name] = result

	return astDefine{def.name, result}, nil
}

func (lt astLet) eval(ctx evalCtx) (ast, error) {
	oldBinds := make(map[astIdent]ast)
	for _, bind := range lt.binds {
		if val, ok := ctx.binds[bind.name]; ok {
			oldBinds[bind.name] = val
		}
	}

	for _, bind := range lt.binds {
		val, err := bind.ast.eval(ctx)
		if err != nil {
			return nil, err
		}

		ctx.binds[bind.name] = val
	}

	result, err := lt.ast.eval(ctx)
	if err != nil {
		return nil, err
	}

	for _, bind := range lt.binds {
		if val, ok := oldBinds[bind.name]; ok {
			ctx.binds[bind.name] = val
		} else {
			delete(ctx.binds, bind.name)
		}
	}

	return result, nil
}

func (ident astIdent) eval(ctx evalCtx) (ast, error) {
	if val, ok := ctx.binds[ident]; ok {
		return val, nil
	} else {
		return nil, fmt.Errorf("unassigned variable: %s", ident)
	}
}

func (str astString) eval(ctx evalCtx) (ast, error) {
	return str, nil
}

func (x astInt) eval(ctx evalCtx) (ast, error) {
	return x, nil
}
