package dsl

import (
	"fmt"
	"sort"
	"strings"
	"text/scanner"
)

/* An abstract syntax tree is the parsed representation of our specification language.
* It can be transformed into its evaluated form my calling the eval() method. */
type ast interface {
	String() string
	eval(*evalCtx) (ast, error)
}

type astLambda struct {
	argNames []astIdent
	do       ast
	ctx      *evalCtx // The evalCtx when the lambda was defined.
}

type astAtom struct {
	astSexp
	index int
}

type astRange struct {
	ident astIdent

	min astFloat
	max astFloat
}

type astList []ast          /* A data list after evaluation. */
type astHashmap map[ast]ast /* A map after evaluation. */

type astSexp struct {
	sexp []ast
	pos  scanner.Position
}

/* The top level is a list of abstract syntax trees, typically populated by define
* statements. */
type astRoot astList

type astModule struct {
	moduleName astString
	body       astRoot
}

type astIdent string /* Identities, i.e. key words, variable names etc. */

/* Atoms. */
type astString string
type astFloat float64
type astInt int
type astBool bool

/* SSH Keys */
type astGithubKey astString
type astPlaintextKey astString

/* Machine configurations */
type astSize astString
type astProvider astString

func (p astProvider) String() string {
	return fmt.Sprintf("(provider %s)", astString(p).String())
}

func (size astSize) String() string {
	return fmt.Sprintf("(size %s)", astString(size).String())
}

func (key astGithubKey) String() string {
	return fmt.Sprintf("(githubKey %s)", astString(key).String())
}

func (key astPlaintextKey) String() string {
	return fmt.Sprintf("(plaintextKey %s)", astString(key).String())
}

func (root astRoot) String() string {
	return fmt.Sprintf("%s", sliceStr(root, "\n"))
}

func (module astModule) String() string {
	return fmt.Sprintf("(module %s %s)", module.moduleName, module.body)
}

func (sexp astSexp) String() string {
	return fmt.Sprintf("(%s)", sliceStr(sexp.sexp, " "))
}

func (list astList) String() string {
	if len(list) == 0 {
		return "(list)"
	}

	return fmt.Sprintf("(list %s)", sliceStr(list, " "))
}

func (h astHashmap) String() string {
	if len(h) == 0 {
		return "(hashmap)"
	}

	keyValues := []string{}
	for key, value := range h {
		keyValues = append(keyValues, fmt.Sprintf("(%s %s)", key.String(), value.String()))
	}
	sort.Strings(keyValues)

	return fmt.Sprintf("(hashmap %s)", strings.Join(keyValues[:], " "))
}

func (ident astIdent) String() string {
	return string(ident)
}

func (str astString) String() string {
	/* Must cast str to string otherwise fmt recurses infinitely. */
	return fmt.Sprintf("\"%s\"", string(str))
}

func (x astFloat) String() string {
	return fmt.Sprintf("%g", x)
}

func (x astInt) String() string {
	return fmt.Sprintf("%d", x)
}

func (b astBool) String() string {
	return fmt.Sprintf("%t", b)
}

func (r astRange) String() string {
	args := []ast{r.min}
	if r.max != 0 {
		args = append(args, r.max)
	}

	return fmt.Sprintf("(%s)", sliceStr(append([]ast{r.ident}, args...), " "))
}

func (l astLambda) String() string {
	var args []ast
	for _, name := range l.argNames {
		args = append(args, name)
	}
	return fmt.Sprintf("(lambda (%s) %s)", sliceStr(args, " "), l.do)
}

func sliceStr(asts []ast, sep string) string {
	slice := []string{}
	for _, elem := range asts {
		slice = append(slice, elem.String())
	}

	return strings.Join(slice, sep)
}
