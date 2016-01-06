package dsl

import (
	"fmt"
	"strings"
)

/* An abstract syntaxt tree is the parsed representation of our specification language.
* It can be transformed into its evaluated form my calling the eval() method. */
type ast interface {
	String() string
	eval(map[astIdent]ast) (ast, error)
}

type astBind struct {
	name astIdent
	ast
}

type astLet struct {
	binds []astBind
	ast   ast
}

type astArith struct {
	name string
	do   func(int, int) int
	args []ast
}

/* The top level is a list of abstract syntax trees, typically populated by define
* statements. */
type astRoot []ast

/* Define creates a global variable definition which is made avariable to the rest of the
* DI system. */
type astDefine astBind

type astListOp []ast /* A data list before evaluation (include the 'list' keyword). */
type astList []ast   /* A data list after evaluation. */

type astIdent string /* Identities, i.e. key words, variable names etc. */

/* Atoms. */
type astString string
type astInt int

func (root astRoot) String() string {
	return fmt.Sprintf("%s", sliceStr(root))
}

func (list astListOp) String() string {
	return astList(append([]ast{astIdent("list")}, list...)).String()
}

func (list astList) String() string {
	return fmt.Sprintf("(%s)", sliceStr(list))
}

func (ident astIdent) String() string {
	return string(ident)
}

func (str astString) String() string {
	/* Must cast str to string otherwise fmt recurses infinitely. */
	return fmt.Sprintf("\"%s\"", string(str))
}

func (x astInt) String() string {
	return fmt.Sprintf("%d", x)
}

func (at astArith) String() string {
	return fmt.Sprintf("(%s %s)", at.name, sliceStr(at.args))
}

func (def astDefine) String() string {
	return fmt.Sprintf("(define %s %s)", def.name, def.ast)
}

func (lt astLet) String() string {
	bindSlice := []string{}
	for _, bind := range lt.binds {
		bindStr := fmt.Sprintf("(%s %s)", bind.name, bind.ast)
		bindSlice = append(bindSlice, bindStr)
	}
	bindStr := strings.Join(bindSlice, " ")
	return fmt.Sprintf("(let (%s) %s)", bindStr, lt.ast)
}

func (root astRoot) eval(binds map[astIdent]ast) (ast, error) {
	results, err := astList(root).eval(binds)
	if err != nil {
		return nil, err
	}
	return astRoot(results.(astList)), nil
}

func (list astListOp) eval(binds map[astIdent]ast) (ast, error) {
	return astList(list).eval(binds)
}

func (list astList) eval(binds map[astIdent]ast) (ast, error) {
	result := []ast{}
	for _, elem := range list {
		evaled, err := elem.eval(binds)
		if err != nil {
			return nil, err
		}
		result = append(result, evaled)
	}

	return astList(result), nil
}

func (ident astIdent) eval(binds map[astIdent]ast) (ast, error) {
	if val, ok := binds[ident]; ok {
		return val, nil
	} else {
		return nil, fmt.Errorf("unassigned variable: %s", ident)
	}
}

func (str astString) eval(binds map[astIdent]ast) (ast, error) {
	return str, nil
}

func (x astInt) eval(binds map[astIdent]ast) (ast, error) {
	return x, nil
}

func (at astArith) eval(binds map[astIdent]ast) (ast, error) {
	args := []int{}
	for _, ast := range at.args {
		evaled, err := ast.eval(binds)
		if err != nil {
		}

		val, ok := evaled.(astInt)
		if !ok {
			return nil, fmt.Errorf("arithmetic on non-integer: \"%s\"", ast)
		}
		args = append(args, int(val))
	}

	/* The parser guarantees that there is at last one arg. */
	total := args[0]
	for _, x := range args[1:] {
		total = at.do(total, x)
	}

	return astInt(total), nil
}

func (def astDefine) eval(binds map[astIdent]ast) (ast, error) {
	if _, ok := binds[def.name]; ok {
		return nil, fmt.Errorf("attempt to redefine: \"%s\"", def.name)
	}

	result, err := def.ast.eval(binds)
	if err != nil {
		return nil, err
	}
	binds[def.name] = result
	return astDefine{def.name, result}, nil
}

func (lt astLet) eval(binds map[astIdent]ast) (ast, error) {
	oldBinds := make(map[astIdent]ast)
	for _, bind := range lt.binds {
		if val, ok := binds[bind.name]; ok {
			oldBinds[bind.name] = val
		}
	}

	for _, bind := range lt.binds {
		val, err := bind.ast.eval(binds)
		if err != nil {
			return nil, err
		}

		binds[bind.name] = val
	}

	result, err := lt.ast.eval(binds)
	if err != nil {
		return nil, err
	}

	for _, bind := range lt.binds {
		if val, ok := oldBinds[bind.name]; ok {
			binds[bind.name] = val
		} else {
			delete(binds, bind.name)
		}
	}

	return result, nil
}

func sliceStr(asts []ast) string {
	slice := []string{}
	for _, elem := range asts {
		slice = append(slice, elem.String())
	}

	return strings.Join(slice, " ")
}
