package dsl

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"text/scanner"
)

var errUnbalancedParens = "unbalanced Parenthesis"

func parse(s scanner.Scanner) (astRoot, error) {
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

	return astRoot(pt), nil
}

func parseText(s *scanner.Scanner, depth int) ([]ast, error) {
	var slice []ast
	for {
		switch s.Scan() {
		case '+', '-', '/', '%', '*', scanner.Ident:
			slice = append(slice, astIdent(s.TokenText()))
		case scanner.Float:
			x, _ := strconv.ParseFloat(s.TokenText(), 64)
			slice = append(slice, astFloat(x))
		case scanner.Int:
			x, _ := strconv.Atoi(s.TokenText())
			slice = append(slice, astInt(x))
		case scanner.String:
			str := strings.Trim(s.TokenText(), "\"")
			slice = append(slice, astString(str))
		case '(':
			// We need to save our position before recursing because the scanner
			// will have moved on by the time the recursive call returns.
			pos := s.Pos()
			sexp, err := parseText(s, depth+1)
			if err != nil {
				return nil, err
			}

			slice = append(slice, astSexp{sexp: sexp, pos: pos})

		case ')':
			if depth == 0 {
				return nil, dslError{s.Pos(), errUnbalancedParens}
			}
			return slice, nil
		case scanner.EOF:
			if depth != 0 {
				return nil, dslError{s.Pos(), errUnbalancedParens}
			}
			return slice, nil

		default:
			return nil, dslError{s.Pos(), fmt.Sprintf("bad element: %s", s.TokenText())}
		}
	}
}
