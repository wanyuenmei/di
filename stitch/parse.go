package stitch

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"text/scanner"
)

var errUnbalancedParens = errors.New("unbalanced Parenthesis")

func parse(s scanner.Scanner) ([]ast, error) {
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

func parseIdent(ident string) ast {
	switch ident {
	case "true":
		return astBool(true)
	case "false":
		return astBool(false)
	default:
		return astIdent(ident)
	}
}

func parseText(s *scanner.Scanner, depth int) ([]ast, error) {
	var slice []ast
	for {
		switch s.Scan() {
		case '+', '-', '/', '%', '*', '=', '<', '>', '!':
			slice = append(slice, parseIdent(s.TokenText()))
		case scanner.Ident:
			ident := s.TokenText()
			// Periods are allowed in package names.
			for s.Peek() == '.' {
				s.Next()
				ident += "."
				if s.Scan() != scanner.Ident {
					return nil, stitchError{pos: s.Pos(),
						err: fmt.Errorf("bad ident name: %s", ident)}
				}
				ident += s.TokenText()
			}
			slice = append(slice, parseIdent(ident))
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
				return nil, stitchError{s.Pos(), errUnbalancedParens}
			}
			return slice, nil
		case scanner.EOF:
			if depth != 0 {
				return nil, stitchError{s.Pos(), errUnbalancedParens}
			}
			return slice, nil

		default:
			return nil, stitchError{s.Pos(), fmt.Errorf("bad element: %s", s.TokenText())}
		}
	}
}
