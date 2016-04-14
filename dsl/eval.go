package dsl

import "fmt"

type evalCtx struct {
	binds       map[astIdent]ast
	labels      map[string]astLabel
	connections map[Connection]struct{}
	containers  *[]*astContainer

	parent *evalCtx
}

func (ctx *evalCtx) globalCtx() *evalCtx {
	if ctx.parent == nil {
		return ctx
	}
	return ctx.parent.globalCtx()
}

func (ctx *evalCtx) deepCopy() *evalCtx {
	var parentCopy *evalCtx
	if ctx.parent != nil {
		parentCopy = ctx.parent.deepCopy()
	}

	bindsCopy := make(map[astIdent]ast)
	for k, v := range ctx.binds {
		bindsCopy[k] = v
	}

	return &evalCtx{
		binds:       bindsCopy,
		labels:      ctx.labels,
		connections: ctx.connections,
		containers:  ctx.containers,
		parent:      parentCopy,
	}
}

func eval(parsed ast) (ast, evalCtx, error) {
	globalCtx := newEvalCtx(nil)

	evaluated, err := parsed.eval(&globalCtx)
	if err != nil {
		return nil, evalCtx{}, err
	}

	return evaluated, globalCtx, nil
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

func evalLambda(fn astLambda, funcArgs []ast) (ast, error) {
	parentCtx := fn.ctx
	if len(fn.argNames) != len(funcArgs) {
		return nil, fmt.Errorf("bad number of arguments: %s %s", fn.argNames, funcArgs)
	}

	// Modify the eval context with the argument binds.
	fnCtx := newEvalCtx(parentCtx)

	for i, ident := range fn.argNames {
		fnArg, err := funcArgs[i].eval(parentCtx)
		if err != nil {
			return nil, err
		}
		fnCtx.binds[ident] = fnArg
	}

	return fn.do.eval(&fnCtx)
}

func (metaSexp astSexp) eval(ctx *evalCtx) (ast, error) {
	sexp := metaSexp.sexp
	if len(sexp) == 0 {
		return nil, dslError{metaSexp.pos, fmt.Sprintf("S-expressions must start with a function call: %s", metaSexp)}
	}

	first, err := sexp[0].eval(ctx)
	if err != nil {
		if _, ok := sexp[0].(astIdent); ok {
			return nil, dslError{metaSexp.pos, fmt.Sprintf("unknown function: %s", sexp[0])}
		}
		return nil, err
	}

	var res ast
	switch fn := first.(type) {
	case astIdent:
		fnImpl := funcImplMap[fn]
		if len(sexp)-1 < fnImpl.minArgs {
			return nil, dslError{metaSexp.pos,
				fmt.Sprintf("not enough arguments: %s", fn)}
		}

		args := sexp[1:]
		if !fnImpl.lazy {
			args, err = evalList(ctx, args)
			if err != nil {
				break
			}
		}
		res, err = fnImpl.do(ctx, args)
	case astLambda:
		var args []ast
		args, err = evalList(ctx, sexp[1:])
		if err != nil {
			break
		}
		res, err = evalLambda(fn, args)
	default:
		return nil, dslError{metaSexp.pos, fmt.Sprintf("S-expressions must start with a function call: %s", first)}
	}

	// Attach the error position if there's an error, and it doesn't already contain
	// the position information.
	if _, ok := err.(dslError); err != nil && !ok {
		err = dslError{metaSexp.pos, err.Error()}
	}
	return res, err
}

func (ident astIdent) eval(ctx *evalCtx) (ast, error) {
	// If the ident represents a built-in function, just return the identifier.
	// S-exp eval will know what to do with it.
	if _, ok := funcImplMap[ident]; ok {
		return ident, nil
	}

	if val, ok := ctx.binds[ident]; ok {
		return val, nil
	} else if ctx.parent == nil {
		return nil, fmt.Errorf("unassigned variable: %s", ident)
	}
	return ident.eval(ctx.parent)
}

func (m astHmap) eval(ctx *evalCtx) (ast, error) {
	return m, nil
}

func (str astString) eval(ctx *evalCtx) (ast, error) {
	return str, nil
}

func (x astFloat) eval(ctx *evalCtx) (ast, error) {
	return x, nil
}

func (x astInt) eval(ctx *evalCtx) (ast, error) {
	return x, nil
}

func (b astBool) eval(ctx *evalCtx) (ast, error) {
	return b, nil
}

func (githubKey astGithubKey) eval(ctx *evalCtx) (ast, error) {
	return githubKey, nil
}

func (plaintextKey astPlaintextKey) eval(ctx *evalCtx) (ast, error) {
	return plaintextKey, nil
}

func (size astSize) eval(ctx *evalCtx) (ast, error) {
	return size, nil
}

func (p astProvider) eval(ctx *evalCtx) (ast, error) {
	return p, nil
}

func (r astRange) eval(ctx *evalCtx) (ast, error) {
	return r, nil
}

func (l astLambda) eval(ctx *evalCtx) (ast, error) {
	return l, nil
}

func (m astModule) eval(ctx *evalCtx) (ast, error) {
	moduleName := string(m.moduleName)

	// Make the parent context include pointers to global structures.
	// Otherwise, commands such as `docker` won't work within the module.
	globalCtx := ctx.globalCtx().deepCopy()
	globalCtx.binds = make(map[astIdent]ast)
	importCtx := newEvalCtx(globalCtx)

	res, err := astList(m.body).eval(&importCtx)
	if err != nil {
		return nil, err
	}

	// Export the binds.
	for k, v := range importCtx.binds {
		if shouldExport(string(k)) {
			ctx.binds[astIdent(moduleName+"."+string(k))] = v
		}
	}

	return astModule{moduleName: m.moduleName, body: res.(astList)}, nil
}

func (l astLabel) eval(ctx *evalCtx) (ast, error) {
	return l, nil
}

func (m *astMachine) eval(ctx *evalCtx) (ast, error) {
	return m, nil
}

func (c *astContainer) eval(ctx *evalCtx) (ast, error) {
	return c, nil
}

func evalList(ctx *evalCtx, args []ast) ([]ast, error) {
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

func newEvalCtx(parent *evalCtx) evalCtx {
	return evalCtx{
		make(map[astIdent]ast),
		make(map[string]astLabel),
		make(map[Connection]struct{}),
		&[]*astContainer{}, parent}
}
