package dsl

import "fmt"

type Atom struct {
	atom   astAtom
	labels []string
}

type evalCtx struct {
	binds   map[astIdent]ast
	defines map[astIdent]ast
	labels  map[string][]*Container

	containers []*Container
}

func eval(parsed ast) (ast, evalCtx, error) {
	ctx := evalCtx{
		make(map[astIdent]ast),
		make(map[astIdent]ast),
		make(map[string][]*Container),
		nil}
	evaluated, err := parsed.eval(&ctx)
	if err != nil {
		return nil, evalCtx{}, err
	}

	return evaluated, ctx, nil
}

func (root astRoot) eval(ctx *evalCtx) (ast, error) {
	results, err := astList(root).eval(ctx)
	if err != nil {
		return nil, err
	}
	return astRoot(results.(astList)), nil
}

func (list astList) eval(ctx *evalCtx) (ast, error) {
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

func (fn astFunc) eval(ctx *evalCtx) (ast, error) {
	return fn.do(ctx, fn.args)
}

func (def astDefine) eval(ctx *evalCtx) (ast, error) {
	if _, ok := ctx.defines[def.ident]; ok {
		return nil, fmt.Errorf("attempt to redefine: \"%s\"", def.ident)
	}

	result, err := def.ast.eval(ctx)
	if err != nil {
		return nil, err
	}

	ctx.defines[def.ident] = result
	ctx.binds[def.ident] = result

	return astDefine{def.ident, result}, nil
}

func (atom astAtom) eval(ctx *evalCtx) (ast, error) {
	if atom.container != nil {
		panic("Not Reached") // This atom has already been evaluated.
	}

	if atom.typ != "docker" {
		return nil, fmt.Errorf("unknown atom type: %s", atom.typ)
	}

	eval, err := atom.arg.eval(ctx)
	if err != nil {
		return nil, err
	}

	arg, ok := eval.(astString)
	if !ok {
		return nil, fmt.Errorf("atom argument must be a string, found: %s", eval)
	}

	container := &Container{string(arg), nil}
	ctx.containers = append(ctx.containers, container)
	return astAtom{atom.typ, eval, container}, nil
}

func (lt astLet) eval(ctx *evalCtx) (ast, error) {
	oldBinds := make(map[astIdent]ast)
	for _, bind := range lt.binds {
		if val, ok := ctx.binds[bind.ident]; ok {
			oldBinds[bind.ident] = val
		}
	}

	for _, bind := range lt.binds {
		val, err := bind.ast.eval(ctx)
		if err != nil {
			return nil, err
		}

		ctx.binds[bind.ident] = val
	}

	result, err := lt.ast.eval(ctx)
	if err != nil {
		return nil, err
	}

	for _, bind := range lt.binds {
		if val, ok := oldBinds[bind.ident]; ok {
			ctx.binds[bind.ident] = val
		} else {
			delete(ctx.binds, bind.ident)
		}
	}

	return result, nil
}

func (ident astIdent) eval(ctx *evalCtx) (ast, error) {
	if val, ok := ctx.binds[ident]; ok {
		return val, nil
	} else {
		return nil, fmt.Errorf("unassigned variable: %s", ident)
	}
}

func (str astString) eval(ctx *evalCtx) (ast, error) {
	return str, nil
}

func (x astInt) eval(ctx *evalCtx) (ast, error) {
	return x, nil
}
