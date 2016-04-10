package dsl

import (
	"bufio"
	"errors"
	"fmt"
	"text/scanner"

	"github.com/NetSys/di/util"
)

func resolveImports(asts []ast, paths []string) ([]ast, error) {
	return resolveImportsRec(asts, paths, nil)
}

func resolveImportsRec(asts []ast, paths, imported []string) ([]ast, error) {
	var newAsts []ast
	top := true // Imports are required to be at the top of the file.

	for _, ast := range asts {
		name := parseImport(ast)
		if name == "" {
			newAsts = append(newAsts, ast)
			top = false
			continue
		}

		if !top {
			return nil, errors.New("import must be begin the module")
		}

		// Check for any import cycles.
		for _, importedModule := range imported {
			if name == importedModule {
				return nil, fmt.Errorf("import cycle: %s",
					append(imported, name))
			}
		}

		var sc scanner.Scanner
		for _, path := range paths {
			modulePath := path + "/" + name + ".spec"
			f, err := util.Open(modulePath)
			if err == nil {
				defer f.Close()
				sc.Filename = modulePath
				sc.Init(bufio.NewReader(f))
				break
			}
		}

		if sc.Filename == "" {
			return nil, fmt.Errorf("unable to open import %s", name)
		}

		parsed, err := parse(sc)
		if err != nil {
			return nil, err
		}

		parsed, err = resolveImportsRec(parsed, paths, append(imported, name))
		if err != nil {
			return nil, err
		}

		module := astModule{body: parsed, moduleName: astString(name)}
		newAsts = append(newAsts, module)
	}

	return newAsts, nil
}

func parseImport(ast ast) string {
	sexp, ok := ast.(astSexp)
	if !ok {
		return ""
	}

	if len(sexp.sexp) < 1 {
		return ""
	}

	ident, ok := sexp.sexp[0].(astIdent)
	if !ok {
		return ""
	}

	if ident != "import" {
		return ""
	}

	name, ok := sexp.sexp[1].(astString)
	if !ok {
		return ""
	}

	return string(name)
}
