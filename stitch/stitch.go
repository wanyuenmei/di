package stitch

import (
	"fmt"
	"text/scanner"

	log "github.com/Sirupsen/logrus"
)

// A Stitch is an abstract representation of the policy language.
type Stitch struct {
	code string
	ctx  evalCtx
}

// A Placement constraint guides where containers may be scheduled, either relative to
// the labels of other containers, or the machine the container will run on.
type Placement struct {
	TargetLabel string
	Rule        Rule
}

// A Rule specifies the specific constraint used in a Placement.
type Rule struct {
	Exclusive         bool
	OtherLabels       []string
	MachineAttributes map[string]string
}

// A Container may be instantiated in the stitch and queried by users.
type Container struct {
	Image   string
	Command []string
	Env     map[string]string

	atomImpl
}

// A Connection allows containers implementing the From label to speak to containers
// implementing the To label in ports in the range [MinPort, MaxPort]
type Connection struct {
	From    string
	To      string
	MinPort int
	MaxPort int
}

// A ConnectionSlice allows for slices of Collections to be used in joins
type ConnectionSlice []Connection

// A Machine specifies the type of VM that should be booted.
type Machine struct {
	Provider string
	Role     string
	Size     string
	CPU      Range
	RAM      Range
	DiskSize int
	Region   string
	SSHKeys  []string

	atomImpl
}

// A Range defines a range of acceptable values for a Machine attribute
type Range struct {
	Min float64
	Max float64
}

// PublicInternetLabel is a magic label that allows connections to or from the public
// network.
const PublicInternetLabel = "public"

// Accepts returns true if `x` is within the range specified by `stitchr` (include),
// or if no max is specified and `x` is larger than `stitchr.min`.
func (stitchr Range) Accepts(x float64) bool {
	return stitchr.Min <= x && (stitchr.Max == 0 || x <= stitchr.Max)
}

// New parses and executes a stitch (in text form), and returns an abstract Stitch
// handle.
func New(sc scanner.Scanner, path []string) (Stitch, error) {
	parsed, err := parse(sc)
	if err != nil {
		return Stitch{}, err
	}

	parsed, err = resolveImports(parsed, path)
	if err != nil {
		return Stitch{}, err
	}

	_, ctx, err := eval(astRoot(parsed))
	if err != nil {
		return Stitch{}, err
	}

	return Stitch{astRoot(parsed).String(), ctx}, nil
}

// QueryContainers retrieves all containers declared in stitch.
func (stitch Stitch) QueryContainers() []*Container {
	var containers []*Container
	for _, c := range *stitch.ctx.containers {
		var command []string
		for _, co := range c.command {
			command = append(command, string(co.(astString)))
		}
		env := make(map[string]string)
		for key, val := range c.env {
			env[string(key.(astString))] = string(val.(astString))
		}
		containers = append(containers, &Container{
			Image:    string(c.image),
			Command:  command,
			atomImpl: c.atomImpl,
			Env:      env,
		})
	}
	return containers
}

func parseKeys(rawKeys []key) []string {
	var keys []string
	for _, val := range rawKeys {
		key, ok := val.(key)
		if !ok {
			log.Warnf("%s: Requested []key, found %s", key, val)
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

func convertAstMachine(machineAst astMachine) Machine {
	return Machine{
		Provider: string(machineAst.provider),
		Size:     string(machineAst.size),
		Role:     string(machineAst.role),
		Region:   string(machineAst.region),
		RAM:      Range{Min: float64(machineAst.ram.min), Max: float64(machineAst.ram.max)},
		CPU:      Range{Min: float64(machineAst.cpu.min), Max: float64(machineAst.cpu.max)},
		DiskSize: int(machineAst.diskSize),
		SSHKeys:  parseKeys(machineAst.sshKeys),
		atomImpl: machineAst.atomImpl,
	}
}

// QueryMachineSlice returns the machines associated with a label.
func (stitch Stitch) QueryMachineSlice(key string) []Machine {
	label, ok := stitch.ctx.labels[key]
	if !ok {
		log.Warnf("%s undefined", key)
		return nil
	}
	result := label.elems

	var machines []Machine
	for _, val := range result {
		machineAst, ok := val.(*astMachine)
		if !ok {
			log.Warnf("%s: Requested []machine, found %s", key, val)
			return nil
		}
		machines = append(machines, convertAstMachine(*machineAst))
	}

	return machines
}

// QueryMachines returns all machines declared in the stitch.
func (stitch Stitch) QueryMachines() []Machine {
	var machines []Machine
	for _, machineAst := range *stitch.ctx.machines {
		machines = append(machines, convertAstMachine(*machineAst))
	}
	return machines
}

// QueryConnections returns the connections declared in the stitch.
func (stitch Stitch) QueryConnections() map[Connection]struct{} {
	copy := map[Connection]struct{}{}
	for c := range stitch.ctx.connections {
		copy[c] = struct{}{}
	}
	return copy
}

// QueryPlacements returns the placements declared in the stitch.
func (stitch Stitch) QueryPlacements() []Placement {
	var placements []Placement
	for _, p := range *stitch.ctx.placements {
		placements = append(placements, p)
	}
	return placements
}

// QueryFloat returns a float value defined in the stitch.
func (stitch Stitch) QueryFloat(key string) (float64, error) {
	result, ok := stitch.ctx.binds[astIdent(key)]
	if !ok {
		return 0, fmt.Errorf("%s undefined", key)
	}

	val, ok := result.(astFloat)
	if !ok {
		return 0, fmt.Errorf("%s: Requested float, found %s", key, val)
	}

	return float64(val), nil
}

// QueryInt returns an integer value defined in the stitch.
func (stitch Stitch) QueryInt(key string) int {
	result, ok := stitch.ctx.binds[astIdent(key)]
	if !ok {
		log.Warnf("%s undefined", key)
		return 0
	}

	val, ok := result.(astInt)
	if !ok {
		log.Warnf("%s: Requested int, found %s", key, val)
		return 0
	}

	return int(val)
}

// QueryString returns a string value defined in the stitch.
func (stitch Stitch) QueryString(key string) string {
	result, ok := stitch.ctx.binds[astIdent(key)]
	if !ok {
		log.Warnf("%s undefined", key)
		return ""
	}

	val, ok := result.(astString)
	if !ok {
		log.Warnf("%s: Requested string, found %s", key, val)
		return ""
	}

	return string(val)
}

// QueryStrSlice returns a string slice value defined in the stitch.
func (stitch Stitch) QueryStrSlice(key string) []string {
	result, ok := stitch.ctx.binds[astIdent(key)]
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

// String returns the stitch in its code form.
func (stitch Stitch) String() string {
	return stitch.code
}

// When an error occurs within a generated S-expression, the position field
// doesn't get set. To circumvent this, we store a traceback of positions, and
// use the innermost defined position to generate our error message.

// For example, our error trace may look like this:
// Line 5		 : Function call failed
// Line 6		 : Apply failed
// Undefined line: `a` undefined.

// By using the innermost defined position, and the innermost error message,
// our error message is "Line 6: `a` undefined", instead of
// "Undefined line: `a` undefined.
type stitchError struct {
	pos scanner.Position
	err error
}

func (stitchErr stitchError) Error() string {
	pos := stitchErr.innermostPos()
	err := stitchErr.innermostError()
	if pos.Filename == "" {
		return fmt.Sprintf("%d: %s", pos.Line, err)
	}
	return fmt.Sprintf("%s:%d: %s", pos.Filename, pos.Line, err)
}

// innermostPos returns the most nested position that is non-zero.
func (stitchErr stitchError) innermostPos() scanner.Position {
	childErr, ok := stitchErr.err.(stitchError)
	if !ok {
		return stitchErr.pos
	}

	innerPos := childErr.innermostPos()
	if innerPos.Line == 0 {
		return stitchErr.pos
	}
	return innerPos
}

func (stitchErr stitchError) innermostError() error {
	switch childErr := stitchErr.err.(type) {
	case stitchError:
		return childErr.innermostError()
	default:
		return childErr
	}
}

// Get returns the value contained at the given index
func (cs ConnectionSlice) Get(ii int) interface{} {
	return cs[ii]
}

// Len returns the number of items in the slice
func (cs ConnectionSlice) Len() int {
	return len(cs)
}
