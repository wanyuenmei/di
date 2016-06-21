package stitch

import (
	"fmt"
	"sort"
	"strings"
	"text/scanner"
)

type atom interface {
	Labels() []string
	SetLabels([]string)

	ast
}

type atomImpl struct {
	labels []string
}

func (l *atomImpl) Labels() []string {
	return l.labels
}

func (l *atomImpl) SetLabels(labels []string) {
	l.labels = labels
}

/* An abstract syntax tree is the parsed representation of our specification language.
* It can be transformed into its evaluated form my calling the eval() method. */
type ast interface {
	String() string
	eval(*evalCtx) (ast, error)
}

type astLambda struct {
	argNames []astIdent
	do       []ast
	ctx      *evalCtx // The evalCtx when the lambda was defined.
}

type astRange struct {
	ident astIdent

	min astFloat
	max astFloat
}

type astList []ast       /* A data list after evaluation. */
type astHmap map[ast]ast /* A map after evaluation. */

type astSexp struct {
	sexp []ast
	pos  scanner.Position
}

type astLabel struct {
	ident astString
	elems []atom
}

/* The top level is a list of abstract syntax trees, typically populated by define
* statements. */
// XXX Should ditch astRoot all together.
type astRoot astList

type astModule struct {
	moduleName astString
	body       []ast
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
type astRole astString
type astRegion astString
type astDiskSize astInt
type astMachine struct {
	provider astProvider
	role     astRole
	size     astSize
	region   astRegion
	cpu      astRange
	ram      astRange
	diskSize astDiskSize
	sshKeys  []key

	atomImpl
}

type astContainer struct {
	image   astString
	command astList
	env     astHmap

	atomImpl
}

type astLabelRule struct {
	otherLabels []astString
	exclusive   astBool
}

type astMachineRule struct {
	exclusive astBool
	provider  astProvider
	region    astRegion
	size      astSize
}

func (r astMachineRule) String() string {
	exclusiveStr := "on"
	if r.exclusive {
		exclusiveStr = "exclusive"
	}

	var tags []string
	if r.provider != "" {
		tags = append(tags, fmt.Sprintf("(provider %s)", r.provider))
	}
	if r.region != "" {
		tags = append(tags, fmt.Sprintf("(region %s)", r.region))
	}
	if r.size != "" {
		tags = append(tags, fmt.Sprintf("(size %s)", r.size))
	}

	return fmt.Sprintf("(machineRule %s %s)", exclusiveStr, strings.Join(tags, " "))
}

func (r astLabelRule) String() string {
	exclusiveStr := "on"
	if r.exclusive {
		exclusiveStr = "exclusive"
	}

	var otherLabels []string
	for _, l := range r.otherLabels {
		otherLabels = append(otherLabels, string(l))
	}

	return fmt.Sprintf("(labelRule %s %s)", exclusiveStr, strings.Join(otherLabels, " "))
}

func (r astRegion) String() string {
	return fmt.Sprintf("(region %s)", astString(r).String())
}

func (p astProvider) String() string {
	return fmt.Sprintf("(provider %s)", astString(p).String())
}

func (size astSize) String() string {
	return fmt.Sprintf("(size %s)", astString(size).String())
}

func (size astDiskSize) String() string {
	return fmt.Sprintf("(diskSize %d)", int(size))
}

func (role astRole) String() string {
	return fmt.Sprintf("(role %s)", astString(role).String())
}

func (key astGithubKey) String() string {
	return fmt.Sprintf("(githubKey %s)", astString(key).String())
}

func (key astPlaintextKey) String() string {
	return fmt.Sprintf("(sshkey %s)", astString(key).String())
}

func (root astRoot) String() string {
	return fmt.Sprintf("%s", sliceStr(root, "\n"))
}

func (module astModule) String() string {
	return fmt.Sprintf("(module %s %s)", module.moduleName,
		sliceStr(module.body, "\n"))
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

func (h astHmap) String() string {
	if len(h) == 0 {
		return "(hmap)"
	}

	keyValues := []string{}
	for key, value := range h {
		keyValues = append(keyValues, fmt.Sprintf("(%s %s)", key.String(), value.String()))
	}
	sort.Strings(keyValues)

	return fmt.Sprintf("(hmap %s)", strings.Join(keyValues[:], " "))
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

func (l astLabel) String() string {
	var asts []ast
	for _, elem := range l.elems {
		asts = append(asts, ast(elem))
	}
	return fmt.Sprintf("(label %s %s)", l.ident, sliceStr(asts, " "))
}

func (m *astMachine) String() string {
	var args []ast
	if m.provider != "" {
		args = append(args, m.provider)
	}
	if m.region != "" {
		args = append(args, m.region)
	}
	if m.size != "" {
		args = append(args, m.size)
	}
	if m.diskSize != 0 {
		args = append(args, m.diskSize)
	}
	if m.ram.ident != "" {
		args = append(args, m.ram)
	}
	if m.cpu.ident != "" {
		args = append(args, m.cpu)
	}
	if m.role != "" {
		args = append(args, m.role)
	}
	for _, key := range m.sshKeys {
		args = append(args, key)
	}
	if len(args) == 0 {
		return "(machine)"
	}
	return fmt.Sprintf("(machine %s)", sliceStr(args, " "))
}

func (c *astContainer) String() string {
	if len(c.command) == 0 {
		return fmt.Sprintf("(docker %s)", c.image)
	}
	if len(c.env) == 0 {
		return fmt.Sprintf("(docker %s %s)", c.image, sliceStr(c.command, " "))
	}
	return fmt.Sprintf("(docker %s %s %s)", c.image, sliceStr(c.command, " "),
		c.env.String())
}

func sliceStr(asts []ast, sep string) string {
	slice := []string{}
	for _, elem := range asts {
		slice = append(slice, elem.String())
	}

	return strings.Join(slice, sep)
}
