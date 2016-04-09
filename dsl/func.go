package dsl

import (
	"bufio"
	"fmt"
	"reflect"
	"strings"
	"text/scanner"
	"unicode"
	"unicode/utf8"

	"github.com/NetSys/di/util"

	log "github.com/Sirupsen/logrus"
)

type funcImpl struct {
	do      func(*evalCtx, []ast) (ast, error)
	minArgs int
}

var funcImplMap map[astIdent]funcImpl

// We have to initialize `funcImplMap` in an init function or else the compiler
// will complain about an initialization loop (funcImplMap -> letImpl ->
// astSexp.eval -> funcImplMap).
func init() {
	funcImplMap = map[astIdent]funcImpl{
		"!":                {notImpl, 1},
		"%":                {arithFun(func(a, b int) int { return a % b }), 2},
		"*":                {arithFun(func(a, b int) int { return a * b }), 2},
		"+":                {arithFun(func(a, b int) int { return a + b }), 2},
		"-":                {arithFun(func(a, b int) int { return a - b }), 2},
		"/":                {arithFun(func(a, b int) int { return a / b }), 2},
		"<":                {compareFun(func(a, b int) bool { return a < b }), 2},
		"=":                {eqImpl, 2},
		">":                {compareFun(func(a, b int) bool { return a > b }), 2},
		"and":              {andImpl, 1},
		"connect":          {connectImpl, 3},
		"cpu":              {rangeImpl("cpu"), 1},
		"define":           {defineImpl, 2},
		"docker":           {dockerImpl, 1},
		"githubKey":        {githubKeyImpl, 1},
		"hashmap":          {hashmapImpl, 0},
		"hashmapGet":       {hashmapGetImpl, 2},
		"hashmapSet":       {hashmapSetImpl, 3},
		"if":               {ifImpl, 2},
		"import":           {importImpl, 1},
		"label":            {labelImpl, 2},
		"lambda":           {lambdaImpl, 2},
		"let":              {letImpl, 2},
		"list":             {listImpl, 0},
		"machine":          {machineImpl, 0},
		"machineAttribute": {machineAttributeImpl, 2},
		"makeList":         {makeListImpl, 2},
		"module":           {moduleImpl, 2},
		"or":               {orImpl, 1},
		"placement":        {placementImpl, 3},
		"plaintextKey":     {plaintextKeyImpl, 1},
		"progn":            {prognImpl, 1},
		"provider":         {providerImpl, 1},
		"ram":              {rangeImpl("ram"), 1},
		"size":             {sizeImpl, 1},
		"sprintf":          {sprintfImpl, 1},
	}
}

// XXX: support float operators?
func arithFun(do func(a, b int) int) func(*evalCtx, []ast) (ast, error) {
	return func(ctx *evalCtx, argsAst []ast) (ast, error) {
		args, err := evalArgs(ctx, argsAst)

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

func eqImpl(ctx *evalCtx, argsAst []ast) (ast, error) {
	args, err := evalArgs(ctx, argsAst)

	if err != nil {
		return nil, err
	}
	return astBool(reflect.DeepEqual(args[0], args[1])), nil
}

func compareFun(do func(a, b int) bool) func(*evalCtx, []ast) (ast, error) {
	return func(ctx *evalCtx, argsAst []ast) (ast, error) {
		args, err := evalArgs(ctx, argsAst)

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

		return astBool(do(ints[0], ints[1])), nil
	}
}

func dockerImpl(ctx *evalCtx, argsAst []ast) (ast, error) {
	evalArgs, err := evalArgs(ctx, argsAst)
	if err != nil {
		return nil, err
	}

	var args []string
	for _, ev := range evalArgs {
		arg, ok := ev.(astString)
		if !ok {
			return nil, fmt.Errorf("docker arguments must be strings: %s",
				ev)
		}
		args = append(args, string(arg))
	}

	var command []string
	if len(args) > 1 {
		command = args[1:]
	}

	container := &Container{
		Image: args[0], Command: command,
		Placement: Placement{make(map[[2]string]struct{})},
	}

	return astAtom{astFunc(astIdent("docker"), evalArgs), addAtom(ctx, container)}, nil
}

func githubKeyImpl(ctx *evalCtx, argsAst []ast) (ast, error) {
	evalArgs, err := evalArgs(ctx, argsAst)
	if err != nil {
		return nil, err
	}
	key := &githubKey{username: string(evalArgs[0].(astString))}

	return astAtom{astFunc(astIdent("githubKey"), evalArgs), addAtom(ctx, key)}, nil
}

func plaintextKeyImpl(ctx *evalCtx, argsAst []ast) (ast, error) {
	evalArgs, err := evalArgs(ctx, argsAst)
	if err != nil {
		return nil, err
	}

	key := &plaintextKey{key: string(evalArgs[0].(astString))}

	return astAtom{astFunc(astIdent("plaintextKey"), evalArgs), addAtom(ctx, key)}, nil
}

func placementImpl(ctx *evalCtx, argsAst []ast) (ast, error) {
	args, err := evalArgs(ctx, argsAst)
	if err != nil {
		return nil, err
	}

	str, ok := args[0].(astString)
	if !ok {
		return nil, fmt.Errorf("placement type must be a string, found: %s", args[0])
	}
	ptype := string(str)

	var labels []string
	for _, arg := range args[1:] {
		str, ok = arg.(astString)
		if !ok {
			return nil, fmt.Errorf("placement arg must be a string, found: %s", arg)
		}
		labels = append(labels, string(str))
	}

	parsedLabels := make(map[[2]string]struct{})
	for i := 0; i < len(labels)-1; i++ {
		for j := i + 1; j < len(labels); j++ {
			if labels[i] < labels[j] {
				parsedLabels[[2]string{labels[i], labels[j]}] = struct{}{}
			} else {
				parsedLabels[[2]string{labels[j], labels[i]}] = struct{}{}
			}
		}
	}

	switch ptype {
	case "exclusive":
		for _, label := range labels {
			for _, c := range ctx.globalCtx().labels[label] {
				c, ok := c.(*Container)
				if !ok {
					return nil, fmt.Errorf("placement labels must contain containers: %s", label)
				}
				for k, v := range parsedLabels {
					c.Placement.Exclusive[k] = v
				}
			}
		}
	default:
		return nil, fmt.Errorf("not a valid placement type: %s", ptype)
	}

	return astFunc(astIdent("placement"), args), nil
}

func setMachineAttributes(machine *Machine, args []ast) error {
	for _, arg := range flatten(args) {
		switch arg.(type) {
		case astProvider:
			machine.Provider = string(arg.(astProvider))
		case astSize:
			machine.Size = string(arg.(astSize))
		case astRange:
			r := arg.(astRange)
			dslr := Range{Min: float64(r.min), Max: float64(r.max)}
			switch string(r.ident) {
			case "ram":
				machine.RAM = dslr
			case "cpu":
				machine.CPU = dslr
			default:
				return fmt.Errorf("unrecognized argument to machine definition: %s", arg)
			}
		default:
			return fmt.Errorf("unrecognized argument to machine definition: %s", arg)
		}
	}
	return nil
}

func machineImpl(ctx *evalCtx, args []ast) (ast, error) {
	evalArgs, err := evalArgs(ctx, args)
	if err != nil {
		return nil, err
	}

	machine := &Machine{}
	err = setMachineAttributes(machine, evalArgs)
	if err != nil {
		return nil, err
	}

	return astAtom{astFunc(astIdent("machine"), evalArgs), addAtom(ctx, machine)}, nil
}

func machineAttributeImpl(ctx *evalCtx, argsAst []ast) (ast, error) {
	evalArgs, err := evalArgs(ctx, argsAst)
	if err != nil {
		return nil, err
	}

	key, ok := evalArgs[0].(astString)
	if !ok {
		return nil, fmt.Errorf("machineAttribute key must be a string: %s", evalArgs[0])
	}

	target, ok := ctx.globalCtx().labels[string(key)]
	if !ok {
		return nil, fmt.Errorf("machineAttribute key not defined: %s", key)
	}

	for _, val := range target {
		machine, ok := val.(*Machine)
		if !ok {
			return nil, fmt.Errorf("bad type, cannot change machine attributes: %s", val)
		}
		err = setMachineAttributes(machine, evalArgs[1:])
		if err != nil {
			return nil, err
		}
	}

	return astFunc(astIdent("machineAttribute"), evalArgs), nil
}

func providerImpl(ctx *evalCtx, args []ast) (ast, error) {
	evalArgs, err := evalArgs(ctx, args)
	if err != nil {
		return nil, err
	}

	return astProvider((evalArgs[0].(astString))), nil
}

func sizeImpl(ctx *evalCtx, args []ast) (ast, error) {
	evalArgs, err := evalArgs(ctx, args)
	if err != nil {
		return nil, err
	}

	return astSize((evalArgs[0].(astString))), nil
}

func toFloat(x ast) (astFloat, error) {
	switch x.(type) {
	case astInt:
		return astFloat(x.(astInt)), nil
	case astFloat:
		return x.(astFloat), nil
	default:
		return astFloat(0), fmt.Errorf("%v is not convertable to a float", x)
	}

}

func rangeImpl(rangeType string) func(*evalCtx, []ast) (ast, error) {
	return func(ctx *evalCtx, args__ []ast) (ast, error) {
		evalArgs, err := evalArgs(ctx, args__)
		if err != nil {
			return nil, err
		}

		var max astFloat
		var maxErr error
		if len(evalArgs) > 1 {
			max, maxErr = toFloat(evalArgs[1])
		}
		min, minErr := toFloat(evalArgs[0])

		if minErr != nil || maxErr != nil {
			return nil, fmt.Errorf("range arguments must be convertable to floats: %v", evalArgs)
		}

		return astRange{ident: astIdent(rangeType), min: min, max: max}, nil
	}
}

func connectImpl(ctx *evalCtx, argsAst []ast) (ast, error) {
	args, err := evalArgs(ctx, argsAst)
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

		if _, ok := ctx.globalCtx().labels[string(label)]; !ok {
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

	return astFunc(astIdent("connect"), newArgs), nil
}

func labelImpl(ctx *evalCtx, argsAst []ast) (ast, error) {
	args, err := evalArgs(ctx, argsAst)
	if err != nil {
		return nil, err
	}

	str, ok := args[0].(astString)
	if !ok {
		return nil, fmt.Errorf("label must be a string, found: %s", args[0])
	}
	label := string(str)
	if label != strings.ToLower(label) {
		log.Error("Labels must be lowercase, sorry! https://github.com/docker/swarm/issues/1795")
	}

	globalCtx := ctx.globalCtx()
	if _, ok := globalCtx.labels[label]; ok {
		return nil, fmt.Errorf("attempt to redefine label: %s", label)
	}

	var atoms []atom
	for _, elem := range flatten(args[1:]) {
		switch t := elem.(type) {
		case astAtom:
			atoms = append(atoms, globalCtx.atoms[t.index])
		case astString:
			children, ok := globalCtx.labels[string(t)]
			if !ok {
				return nil, fmt.Errorf("undefined label: %s", t)
			}

			for _, c := range children {
				atoms = append(atoms, c)
			}
		default:
			return nil, fmt.Errorf("label must apply to atoms or other"+
				" labels, found: %s", elem)
		}
	}

	for _, a := range atoms {
		labels := a.Labels()
		if len(labels) > 0 && labels[len(labels)-1] == label {
			// It's possible that the same container appears in the list
			// twice.  If that's the case, we'll end up labelling it multiple
			// times unless we check it's most recently added label.
			continue
		}

		a.SetLabels(append(labels, label))
	}

	globalCtx.labels[label] = atoms

	return astFunc(astIdent("label"), args), nil
}

func listImpl(ctx *evalCtx, argsAst []ast) (ast, error) {
	args, err := evalArgs(ctx, argsAst)
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

func hashmapImpl(ctx *evalCtx, args []ast) (ast, error) {
	m := astHashmap(make(map[ast]ast))
	bindings, err := parseBindings(ctx, astSexp{sexp: args})
	if err != nil {
		return nil, err
	}
	for _, bind := range bindings {
		key, err := bind.key.eval(ctx)
		if err != nil {
			return nil, err
		}
		val, err := bind.val.eval(ctx)
		if err != nil {
			return nil, err
		}
		m[key] = val
	}
	return m, nil
}

func hashmapSetImpl(ctx *evalCtx, args []ast) (ast, error) {
	args, err := evalArgs(ctx, args)
	if err != nil {
		return nil, err
	}

	m, ok := args[0].(astHashmap)
	if !ok {
		return nil, fmt.Errorf("%s must be a hashmap", args[0])
	}
	newM := astHashmap(make(map[ast]ast))
	for k, v := range m {
		newM[k] = v
	}
	newM[args[1]] = args[2]
	return newM, nil
}

func hashmapGetImpl(ctx *evalCtx, args []ast) (ast, error) {
	args, err := evalArgs(ctx, args)
	if err != nil {
		return nil, err
	}

	m, ok := args[0].(astHashmap)
	if !ok {
		return nil, fmt.Errorf("%s must be a hashmap", args[0])
	}
	value, ok := m[args[1]]
	if !ok {
		return nil, fmt.Errorf("undefined key: %s", args[1])
	}
	return value, nil
}

func sprintfImpl(ctx *evalCtx, argsAst []ast) (ast, error) {
	args, err := evalArgs(ctx, argsAst)
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

func defineImpl(ctx *evalCtx, args []ast) (ast, error) {
	ident, ok := args[0].(astIdent)
	if !ok {
		return nil, fmt.Errorf("define name must be an ident: %s", args[0])
	}

	if _, ok := ctx.binds[ident]; ok {
		return nil, fmt.Errorf("attempt to redefine: \"%s\"", args[0].(astIdent))
	}

	result, err := args[1].eval(ctx)
	if err != nil {
		return nil, err
	}

	ctx.binds[ident] = result

	return astFunc(astIdent("define"), []ast{ident, result}), nil
}

type binding struct {
	key ast
	val ast
}

func parseBindings(ctx *evalCtx, bindings astSexp) ([]binding, error) {
	var binds []binding
	for _, astBinding := range bindings.sexp {
		bind, ok := astBinding.(astSexp)
		if !ok || len(bind.sexp) != 2 {
			return nil, fmt.Errorf("binds must be exactly 2 arguments: %s", astBinding)
		}
		binds = append(binds, binding{bind.sexp[0], bind.sexp[1]})
	}
	return binds, nil
}

func lambdaImpl(ctx *evalCtx, args []ast) (ast, error) {
	rawArgNames, ok := args[0].(astSexp)
	if !ok {
		return nil, fmt.Errorf("lambda functions must define an argument list")
	}

	var argNames []astIdent
	for _, argName := range rawArgNames.sexp {
		ident, ok := argName.(astIdent)
		if !ok {
			return nil, fmt.Errorf("lambda argument names must be idents")
		}
		argNames = append(argNames, ident)
	}
	return astLambda{argNames: argNames, do: args[1], ctx: ctx}, nil
}

func letImpl(ctx *evalCtx, args []ast) (ast, error) {
	bindingsRaw, ok := args[0].(astSexp)
	if !ok {
		return nil, fmt.Errorf("let binds must be defined in an S-expression")
	}

	bindings, err := parseBindings(ctx, bindingsRaw)
	if err != nil {
		return nil, err
	}

	var names []astIdent
	var vals []ast
	for _, pair := range bindings {
		key, ok := pair.key.(astIdent)
		if !ok {
			return nil, fmt.Errorf("bind name must be an ident: %s", pair.key)
		}
		val, err := pair.val.eval(ctx)
		if err != nil {
			return nil, err
		}
		names = append(names, key)
		vals = append(vals, val)
	}
	progn := astSexp{sexp: append([]ast{astIdent("progn")}, args[1:]...)}
	let := astSexp{sexp: append([]ast{astLambda{argNames: names, do: progn, ctx: ctx}}, vals...)}
	return let.eval(ctx)
}

func ifImpl(ctx *evalCtx, args []ast) (ast, error) {
	predAst, err := args[0].eval(ctx)
	if err != nil {
		return nil, err
	}

	pred, ok := predAst.(astBool)
	if !ok {
		return nil, fmt.Errorf("if predicate must be a boolean: %s", predAst)
	}

	if bool(pred) {
		return args[1].eval(ctx)
	}

	// If the predicate is false, but there's no else case.
	if len(args) == 2 {
		return pred, nil
	}
	return args[2].eval(ctx)
}

func andImpl(ctx *evalCtx, args []ast) (ast, error) {
	for _, arg := range args {
		predAst, err := arg.eval(ctx)
		if err != nil {
			return nil, err
		}

		pred, ok := predAst.(astBool)
		if !ok {
			return nil, fmt.Errorf("and predicate must be a boolean: %s", predAst)
		}

		if !pred {
			return astBool(false), nil
		}
	}
	return astBool(true), nil
}

func orImpl(ctx *evalCtx, args []ast) (ast, error) {
	for _, arg := range args {
		predAst, err := arg.eval(ctx)
		if err != nil {
			return nil, err
		}

		pred, ok := predAst.(astBool)
		if !ok {
			return nil, fmt.Errorf("and predicate must be a boolean: %s", predAst)
		}

		if pred {
			return astBool(true), nil
		}
	}
	return astBool(false), nil
}

func notImpl(ctx *evalCtx, args []ast) (ast, error) {
	predAst, err := args[0].eval(ctx)
	if err != nil {
		return nil, err
	}

	pred, ok := predAst.(astBool)
	if !ok {
		return nil, fmt.Errorf("and predicate must be a boolean: %s", predAst)
	}
	return astBool(!pred), nil
}

func shouldExport(name string) bool {
	r, _ := utf8.DecodeRuneInString(name)
	return unicode.IsUpper(r)
}

func importImpl(ctx *evalCtx, args []ast) (ast, error) {
	astModuleName, ok := args[0].(astString)
	if !ok {
		return nil, fmt.Errorf("import name must be a string: %s", args[0])
	}
	moduleName := string(astModuleName)

	// Try to find the import in our path.
	var sc scanner.Scanner
	for _, path := range ctx.path {
		modulePath := path + "/" + moduleName + ".spec"
		f, err := util.Open(modulePath)
		if err == nil {
			defer f.Close()
			sc.Filename = modulePath
			sc.Init(bufio.NewReader(f))
			break
		}
	}
	if sc.Filename == "" {
		return nil, fmt.Errorf("unable to open import %s", moduleName)
	}

	parsed, err := parse(sc)
	if err != nil {
		return nil, err
	}

	return astModule{body: parsed, moduleName: astModuleName}.eval(ctx)
}

func moduleImpl(ctx *evalCtx, args []ast) (ast, error) {
	moduleName, err := args[0].eval(ctx)
	if err != nil {
		return nil, err
	}

	moduleNameStr, ok := moduleName.(astString)
	if !ok {
		return nil, fmt.Errorf("module name must be a string: %s", moduleName)
	}

	return astModule{moduleName: moduleNameStr, body: astRoot(args[1:])}.eval(ctx)
}

func prognImpl(ctx *evalCtx, args []ast) (ast, error) {
	var res ast
	var err error
	for _, arg := range args {
		res, err = arg.eval(ctx)
		if err != nil {
			return nil, err
		}
	}
	return res, nil
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

func astFunc(ident astIdent, args []ast) astSexp {
	return astSexp{sexp: append([]ast{ident}, args...)}
}

// addAtom adds `a` to the global context. We have to place all atoms into
// the same context because we extract them according to their index within
// the atoms list.
// If we didn't use the global scope, then if we created an atom (index 0) in one context,
// and then created another atom (also index 0) in a lambda function, and then tried to reference
// the first atom, we would actually retrieve the second one because of the indexing
// conflict.
func addAtom(ctx *evalCtx, a atom) int {
	globalCtx := ctx.globalCtx()
	globalCtx.atoms = append(globalCtx.atoms, a)
	return len(globalCtx.atoms) - 1
}
