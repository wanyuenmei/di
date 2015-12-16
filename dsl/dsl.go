package dsl

import (
	"io"

	"github.com/op/go-logging"
)

var log = logging.MustGetLogger("dsl")

type Dsl struct {
	defines map[astIdent]ast
	spec    ast
}

func New(reader io.Reader) (Dsl, error) {
	parsed, err := parse(reader)
	if err != nil {
		return Dsl{}, err
	}

	defines := make(map[astIdent]ast)
	spec, err := parsed.eval(defines)
	if err != nil {
		return Dsl{}, err
	}

	return Dsl{defines, spec}, nil
}

func (dsl Dsl) QueryInt(key string) int {
	result, ok := dsl.defines[astIdent(key)]
	if !ok {
		log.Warning("%s undefined", key)
		return 0
	}

	if val, ok := result.(astInt); ok {
		return int(val)
	} else {
		log.Warning("%s: Requested int, found %s", key, val)
		return 0
	}
}

func (dsl Dsl) QueryString(key string) string {
	result, ok := dsl.defines[astIdent(key)]
	if !ok {
		log.Warning("%s undefined", key)
		return ""
	}

	if val, ok := result.(astString); ok {
		return string(val)
	} else {
		log.Warning("%s: Requested string, found %s", key, val)
		return ""
	}
}

func (dsl Dsl) QueryStrSlice(key string) []string {
	result, ok := dsl.defines[astIdent(key)]
	if !ok {
		log.Warning("%s undefined", key)
		return nil
	}

	val, ok := result.(astList)
	if !ok {
		log.Warning("%s: Requested []string, found %s", key, val)
		return nil
	}

	slice := []string{}
	for _, val := range val {
		str, ok := val.(astString)
		if !ok {
			log.Warning("%s: Requested []string, found %s", key, val)
			return nil
		}
		slice = append(slice, string(str))
	}

	return slice
}
