package dsl

import "fmt"

type evalCtx struct {
	binds       map[astIdent]ast
	labels      map[string][]atom
	connections map[Connection]struct{}
	atoms       []atom
}

type atom interface {
	Labels() []string
	SetLabels([]string)
}

type atomImpl struct {
	labels []string
}

func (l *atomImpl) Labels() []string {
	return l.labels
}

func (l *atomImpl) SetLabels(labels []string) {
	l.labels = labels
}

func eval(parsed ast) (ast, evalCtx, error) {
	ctx := evalCtx{
		make(map[astIdent]ast),
		make(map[string][]atom),
		make(map[Connection]struct{}),
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

func (sexp astSexp) eval(ctx *evalCtx) (ast, error) {
	if len(sexp) == 0 {
		return nil, fmt.Errorf("S-expressions must start with a function call: %s", sexp)
	}

	first, err := sexp[0].eval(ctx)
	if err != nil {
		if _, ok := sexp[0].(astIdent); ok {
			return nil, fmt.Errorf("unknown function: %s", sexp[0])
		}
		return nil, err
	}

	switch fn := first.(type) {
	case astIdent:
		fnImpl := funcImplMap[fn]
		if len(sexp)-1 < fnImpl.minArgs {
			return nil, fmt.Errorf("not enough arguments: %s", fn)
		}
		return fnImpl.do(ctx, sexp[1:])
	}

	return nil, fmt.Errorf("S-expressions must start with a function call: %s", first)
}

func (ident astIdent) eval(ctx *evalCtx) (ast, error) {
	// If the ident represents a built-in function, just return the identifier.
	// S-exp eval will know what to do with it.
	if _, ok := funcImplMap[ident]; ok {
		return ident, nil
	}

	val, ok := ctx.binds[ident]
	if !ok {
		return nil, fmt.Errorf("unassigned variable: %s", ident)
	}
	return val, nil
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
