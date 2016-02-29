package dsl

import (
	"io"

	log "github.com/Sirupsen/logrus"
)

type Dsl struct {
	spec ast
	ctx  evalCtx
}

type Container struct {
	Image   string
	Command []string

	Placement
	AtomImpl
}

type Placement struct {
	Exclusive map[[2]string]struct{}
}

type Connection struct {
	From    string
	To      string
	MinPort int
	MaxPort int
}

func New(reader io.Reader) (Dsl, error) {
	parsed, err := parse(reader)
	if err != nil {
		return Dsl{}, err
	}

	spec, ctx, err := eval(parsed)
	if err != nil {
		return Dsl{}, err
	}
	return Dsl{spec, ctx}, nil
}

func (dsl Dsl) QueryContainers() []*Container {
	var containers []*Container
	for _, atom := range dsl.ctx.atoms {
		switch atom.(type) {
		case *Container:
			containers = append(containers, atom.(*Container))
		}
	}
	return containers
}

func (dsl Dsl) QueryKeySlice(key string) []string {
	result, ok := dsl.ctx.labels[key]
	if !ok {
		log.Warnf("%s undefined", key)
		return nil
	}

	var keys []string
	for _, val := range result {
		key, ok := val.(Key)
		if !ok {
			log.Warning("%s: Requested []key, found %s", key, val)
			continue
		}

		parsedKeys, err := key.keys()
		if err != nil {
			log.WithFields(log.Fields{
				"error": err,
				"key":   key,
			}).Warning("Failed to retrieve key.")
			continue
		}

		keys = append(keys, parsedKeys...)
	}

	return keys
}

func (dsl Dsl) QueryConnections() []Connection {
	var connections []Connection
	for c := range dsl.ctx.connections {
		connections = append(connections, c)
	}
	return connections
}

func (dsl Dsl) QueryInt(key string) int {
	result, ok := dsl.ctx.defines[astIdent(key)]
	if !ok {
		log.Warnf("%s undefined", key)
		return 0
	}

	if val, ok := result.(astInt); ok {
		return int(val)
	} else {
		log.Warnf("%s: Requested int, found %s", key, val)
		return 0
	}
}

func (dsl Dsl) QueryString(key string) string {
	result, ok := dsl.ctx.defines[astIdent(key)]
	if !ok {
		log.Warnf("%s undefined", key)
		return ""
	}

	if val, ok := result.(astString); ok {
		return string(val)
	} else {
		log.Warnf("%s: Requested string, found %s", key, val)
		return ""
	}
}

func (dsl Dsl) QueryStrSlice(key string) []string {
	result, ok := dsl.ctx.defines[astIdent(key)]
	if !ok {
		log.Warnf("%s undefined", key)
		return nil
	}

	val, ok := result.(astList)
	if !ok {
		log.Warnf("%s: Requested []string, found %s", key, val)
		return nil
	}

	slice := []string{}
	for _, val := range val {
		str, ok := val.(astString)
		if !ok {
			log.Warnf("%s: Requested []string, found %s", key, val)
			return nil
		}
		slice = append(slice, string(str))
	}

	return slice
}

func (dsl Dsl) String() string {
	return dsl.spec.String()
}
