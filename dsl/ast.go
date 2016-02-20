package dsl

import (
	"fmt"
	"strings"
)

/* An abstract syntax tree is the parsed representation of our specification language.
* It can be transformed into its evaluated form my calling the eval() method. */
type ast interface {
	String() string
	eval(*evalCtx) (ast, error)
}

type astBind struct {
	ident astIdent
	ast
}

type astLet struct {
	binds []astBind
	ast   ast
}

type astFunc struct {
	ident astIdent
	do    func(*evalCtx, []ast) (ast, error)
	args  []ast
}

type astAtom struct {
	astFunc
	index int
}

type astList []ast /* A data list after evaluation. */

/* The top level is a list of abstract syntax trees, typically populated by define
* statements. */
type astRoot astList

/* Define creates a global variable definition which is made avariable to the rest of the
* DI system. */
type astDefine astBind

type astIdent string /* Identities, i.e. key words, variable names etc. */

/* Atoms. */
type astString string
type astInt int

type astGithubKey astString
type astPlaintextKey astString

func (key astGithubKey) String() string {
	return fmt.Sprintf("(githubKey %s)", astString(key).String())
}

func (key astPlaintextKey) String() string {
	return fmt.Sprintf("(plaintextKey %s)", astString(key).String())
}

func (root astRoot) String() string {
	return fmt.Sprintf("%s", sliceStr(root, "\n"))
}

func (list astList) String() string {
	if len(list) == 0 {
		return "(list)"
	}

	return fmt.Sprintf("(list %s)", sliceStr(list, " "))
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

func (fn astFunc) String() string {
	return fmt.Sprintf("(%s)", sliceStr(append([]ast{fn.ident}, fn.args...), " "))
}

func (def astDefine) String() string {
	return fmt.Sprintf("(define %s %s)", def.ident, def.ast)
}

func (lt astLet) String() string {
	bindSlice := []string{}
	for _, bind := range lt.binds {
		bindStr := fmt.Sprintf("(%s %s)", bind.ident, bind.ast)
		bindSlice = append(bindSlice, bindStr)
	}
	bindStr := strings.Join(bindSlice, " ")
	return fmt.Sprintf("(let (%s) %s)", bindStr, lt.ast)
}

func sliceStr(asts []ast, sep string) string {
	slice := []string{}
	for _, elem := range asts {
		slice = append(slice, elem.String())
	}

	return strings.Join(slice, sep)
}
