package dsl

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"text/scanner"
)

var ErrUnbalancedParens = errors.New("unbalanced Parenthesis")
var ErrBinding = errors.New("error parsing bindings")

func parse(reader io.Reader) (astRoot, error) {
	var s scanner.Scanner
	s.Init(reader)

	scanErrors := []string{}
	s.Error = func(s *scanner.Scanner, msg string) {
		scanErrors = append(scanErrors, msg)
	}

	pt, err := parseText(&s, 0)
	if s.ErrorCount != 0 {
		return nil, errors.New(strings.Join(scanErrors, "\n"))
	} else if err != nil {
		return nil, err
	}

	var root []ast
	for _, iface := range pt {
		p, err := parseInterface(iface)
		if err != nil {
			return nil, err
		}
		root = append(root, p)
	}

	return astRoot(root), nil
}

func parseText(s *scanner.Scanner, depth int) ([]interface{}, error) {
	slice := []interface{}{}
	for {
		switch s.Scan() {
		case '+', '-', '/', '%', '*', scanner.Ident:
			slice = append(slice, astIdent(s.TokenText()))

		case scanner.Int:
			x, _ := strconv.Atoi(s.TokenText())
			slice = append(slice, astInt(x))
		case scanner.String:
			str := strings.Trim(s.TokenText(), "\"")
			slice = append(slice, astString(str))
		case '(':
			sexp, err := parseText(s, depth+1)
			if err != nil {
				return nil, err
			}

			slice = append(slice, sexp)

		case ')':
			if depth == 0 {
				return nil, ErrUnbalancedParens
			}
			return slice, nil
		case scanner.EOF:
			if depth != 0 {
				return nil, ErrUnbalancedParens
			}
			return slice, nil

		default:
			return nil, fmt.Errorf("bad element: %s", s.TokenText())
		}
	}
}

func parseInterface(p1 interface{}) (ast, error) {
	var list []interface{}
	switch elem := p1.(type) {
	case []interface{}:
		list = elem
	case astInt:
		return elem, nil
	case astIdent:
		return elem, nil
	case astString:
		return elem, nil
	default:
		return nil, errors.New(fmt.Sprintf("bad element: %s", elem))
	}

	if len(list) == 0 {
		return nil, errors.New(fmt.Sprintf("bad element: %s", list))
	}

	first, ok := list[0].(astIdent)
	if !ok {
		return nil, errors.New("expressions must begin with keywords")
	}

	switch first {
	case "let":
		if len(list) != 3 {
			return nil, errors.New(fmt.Sprintf(
				"not enough arguments: %s", list))
		}

		binds, err := parseBindList(list[1])
		if err != nil {
			return nil, err
		}

		tree, err := parseInterface(list[2])
		if err != nil {
			return nil, err
		}

		return astLet{binds, tree}, nil
	case "define":
		bind, err := parseBind(list[1:])
		if err != nil {
			return nil, err
		}
		return astDefine(bind), nil
	case "atom":
		bind, err := parseBind(list[1:])
		if err != nil {
			return nil, err
		}
		return astAtom{bind.ident, bind.ast, 0}, nil
	default:
		return parseFunc(first, list[1:])
	}
}

func parseFunc(fn astIdent, ifaceArgs []interface{}) (ast, error) {
	fni, ok := funcImplMap[fn]
	if !ok {
		return nil, fmt.Errorf("unknown function: %s", fn)
	}

	if len(ifaceArgs) < fni.minArgs {
		return nil, fmt.Errorf("not enough arguments: %s", fn)
	}

	var args []ast
	for _, arg := range ifaceArgs {
		eval, err := parseInterface(arg)
		if err != nil {
			return nil, err
		}
		args = append(args, eval)
	}

	return astFunc{fn, fni.do, args}, nil
}

func parseBindList(bindIface interface{}) ([]astBind, error) {
	list, ok := bindIface.([]interface{})
	if !ok {
		return nil, ErrBinding
	}

	result := []astBind{}
	for _, elem := range list {
		bind, err := parseBind(elem)
		if err != nil {
			return nil, err
		}
		result = append(result, bind)
	}

	return result, nil
}

func parseBind(iface interface{}) (astBind, error) {
	pair, ok := iface.([]interface{})
	if !ok {
		return astBind{}, ErrBinding
	}

	if len(pair) != 2 {
		return astBind{}, ErrBinding
	}

	ident, ok := pair[0].(astIdent)
	if !ok {
		return astBind{}, ErrBinding
	}

	tree, err := parseInterface(pair[1])
	if err != nil {
		return astBind{}, err
	}

	return astBind{ident, tree}, nil
}
