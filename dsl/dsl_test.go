package dsl

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"text/scanner"

	"github.com/NetSys/di/util"

	"github.com/davecgh/go-spew/spew"
	"github.com/spf13/afero"
)

func TestRange(t *testing.T) {
	parseTestNoPath(t, "(range 0)", "(list)")
	parseTestNoPath(t, "(range 1)", "(list 0)")
	parseTestNoPath(t, "(range 2)", "(list 0 1)")

	parseTestNoPath(t, "(range 1 1)", "(list)")
	parseTestNoPath(t, "(range 1 2)", "(list 1)")
	parseTestNoPath(t, "(range 1 3)", "(list 1 2)")

	parseTestNoPath(t, "(range 0 5 2)", "(list 0 2 4)")
	parseTestNoPath(t, "(range 0 1 2)", "(list 0)")

	parseTestNoPath(t, "(map (lambda (x) (+ 1 x)) (range 3))", "(list 1 2 3)")
}

func TestArith(t *testing.T) {
	parseTestNoPath(t, "2", "2")

	parseTestNoPath(t, "(+ 5 3)", "8")
	parseTestNoPath(t, "(+ 5 3 10)", "18")

	parseTestNoPath(t, "(- 100 50)", "50")
	parseTestNoPath(t, "(- 100 50 100)", "-50")

	parseTestNoPath(t, "(* 2 6)", "12")
	parseTestNoPath(t, "(* 2 6 3)", "36")

	parseTestNoPath(t, "(/ 100 10)", "10")
	parseTestNoPath(t, "(/ 100 10 5)", "2")

	parseTestNoPath(t, "(% 10 100)", "10")
	parseTestNoPath(t, "(% 10 100 3)", "1")

	parseTestNoPath(t, "(+ (* 3 (- 10 2)) (/ 100 (* 25 2)))", "26")
}

func TestStrings(t *testing.T) {
	code := `"foo"`
	parseTestNoPath(t, code, code)

	code = "\"foo\"\n\"bar\""
	parseTestNoPath(t, code, code)

	code = "\"foo\"\n5\n\"bar\""
	parseTestNoPath(t, code, code)

	code = `(sprintf "foo")`
	parseTestNoPath(t, code, `"foo"`)

	code = `(sprintf "%s %s" "foo" "bar")`
	parseTestNoPath(t, code, `"foo bar"`)

	code = `(sprintf "%s %d" "foo" 3)`
	parseTestNoPath(t, code, `"foo 3"`)

	code = `(sprintf "%s %s" "foo" (list 1 2 3))`
	parseTestNoPath(t, code, `"foo (list 1 2 3)"`)

	runtimeErrNoPath(t, "(sprintf a)", "1: unassigned variable: a")
	runtimeErrNoPath(t, "(sprintf 1)", "1: sprintf format must be a string: 1")
}

func TestLet(t *testing.T) {
	parseTestNoPath(t, "(let ((a 5)) a)", "5")
	parseTestNoPath(t, "(+ 12 (let ((a 5)) a))", "17")
	parseTestNoPath(t, "(let ((a 5)) (* a a))", "25")
	parseTestNoPath(t, "(let ((a 5) (b 2)) (* a b))", "10")
	parseTestNoPath(t, "(let ((a 2)) (* (let ((a 3)) (* a a)) (* a a)))", "36")

	// Test the implicit progn
	parseTestNoPath(t, "(let ((a 2)) (define b 3) (* a b))", "6")
}

func TestApply(t *testing.T) {
	parseTestNoPath(t, "(apply + (list 3 5))", "8")
	parseTestNoPath(t,
		"(apply (lambda (x y) (+ x y)) (let ((x 3)) (list x 5)))", "8")
}

func TestLambda(t *testing.T) {
	// Single argument lambda
	parseTestNoPath(t, "((lambda (x) (* x x)) 5)", "25")

	// Two argument lambda
	parseTestNoPath(t, "((lambda (x y) (* x y)) 5 6)", "30")

	parseTestNoPath(t, "((lambda (x y) (* 2 (+ x y))) 5 6)", "22")

	// Unevaluated argument
	parseTestNoPath(t, "((lambda (x y) (+ x y)) 5 (* 2 6))", "17")

	// Named lambda
	squareDef := "(define Square (lambda (x) (* x x)))\n"
	parseTestNoPath(t, squareDef+"(Square 5)", squareDef+"25")

	// Named lambda in let
	squareDef = "(let ((Square (lambda (x) (* x x))))\n"
	parseTestNoPath(t, "(let ((Square (lambda (x) (* x x)))) (Square 6))", "36")

	// Two named lambdas
	cubeDef := "(define Square (lambda (x) (* x x)))\n" +
		"(define Cube (lambda (x) (* x (Square x))))\n"
	parseTestNoPath(t, cubeDef+"(Cube 5)", cubeDef+"125")

	// Test closure
	adder := "(define nAdder (lambda (n) (lambda (x) (+ n x))))\n" +
		"(define fiveAdder (nAdder 5))\n" +
		"(fiveAdder 10)"
	adderRes := "(define nAdder (lambda (n) (lambda (x) (+ n x))))\n" +
		"(define fiveAdder (lambda (x) (+ n x)))\n" +
		"15"
	parseTestNoPath(t, adder, adderRes)

	// Test variable masking
	adder = "((let ((x 5)) (lambda (x) (+ x 1))) 1)"
	parseTestNoPath(t, adder, "2")
}

func TestProgn(t *testing.T) {
	// Test that first argument gets run
	runtimeErrNoPath(t, `(progn (+ 1 "1") (+ 2 2))`, `1: bad arithmetic argument: "1"`)

	// Test that only the second arguments gets returned
	parseTestNoPath(t, "(progn (+ 1 1) (+ 2 2))", "4")
}

func TestIf(t *testing.T) {
	// Test boolean atoms
	trueTest := "(if true 1 0)"
	parseTestNoPath(t, trueTest, "1")

	falseTest := "(if false 1 0)"
	parseTestNoPath(t, falseTest, "0")

	// Test no else case
	falseTest = "(if false 1)"
	parseTestNoPath(t, falseTest, "false")

	// Test one-argument and
	trueTest = "(if (and true) 1 0)"
	parseTestNoPath(t, trueTest, "1")

	falseTest = "(if (and false) 1 0)"
	parseTestNoPath(t, falseTest, "0")

	// Test one-argument or
	trueTest = "(if (or true) 1 0)"
	parseTestNoPath(t, trueTest, "1")

	falseTest = "(if (or false) 1 0)"
	parseTestNoPath(t, falseTest, "0")

	// Test two-argument and
	trueTest = "(if (and true true) 1 0)"
	parseTestNoPath(t, trueTest, "1")

	falseTest = "(if (and true false) 1 0)"
	parseTestNoPath(t, falseTest, "0")

	// Test two-argument or
	trueTest = "(if (or false true) 1 0)"
	parseTestNoPath(t, trueTest, "1")

	falseTest = "(if (or false false) 1 0)"
	parseTestNoPath(t, falseTest, "0")

	// Test short-circuiting
	andShort := "(if (and false (/ 1 0)) 1 0)"
	parseTestNoPath(t, andShort, "0")

	orShort := "(if (or true (/ 1 0)) 1 0)"
	parseTestNoPath(t, orShort, "1")

	// Test = on ints
	compTest := "(= 1 1)"
	parseTestNoPath(t, compTest, "true")

	compTest = "(= 1 2)"
	parseTestNoPath(t, compTest, "false")

	// Test = on strings
	compTest = `(= "hello" "hello")`
	parseTestNoPath(t, compTest, "true")

	compTest = `(= "hello" "world")`
	parseTestNoPath(t, compTest, "false")

	// Test = on lists
	compTest = `(= (list "hello" "world") (list "hello" "world"))`
	parseTestNoPath(t, compTest, "true")

	compTest = `(= (list "hello" "world") (list "world hello"))`
	parseTestNoPath(t, compTest, "false")

	// Test <
	compTest = "(< 2 1)"
	parseTestNoPath(t, compTest, "false")

	compTest = "(< 1 2)"
	parseTestNoPath(t, compTest, "true")

	// Test >
	compTest = "(> 1 2)"
	parseTestNoPath(t, compTest, "false")

	compTest = "(> 2 1)"
	parseTestNoPath(t, compTest, "true")

	// Test !
	notTest := "(! false)"
	parseTestNoPath(t, notTest, "true")

	notTest = "(! true)"
	parseTestNoPath(t, notTest, "false")

	// Putting it together..
	fibDef := "(define fib (lambda (n) (if (= n 0) 1 (* n (fib (- n 1))))))"
	parseTestNoPath(t, fibDef+"\n"+"(fib 5)", fibDef+"\n"+"120")
}

func TestDefine(t *testing.T) {
	code := "(define a 1)"
	parseTestNoPath(t, code, code)

	code = "(define a 1)\n(define b 2)"
	parseTestNoPath(t, code, code)

	code = "(define a 1)\n3\n(define b 2)"
	parseTestNoPath(t, code, code)

	parseTestNoPath(t, "(define a (+ 5 7))", "(define a 12)")

	parseTestNoPath(t, "(define a (+ 5 7))", "(define a 12)")

	parseTestNoPath(t, "(define a (+ 1 1))\n(define b (* a 2))",
		"(define a 2)\n(define b 4)")
}

func TestList(t *testing.T) {
	parseTestNoPath(t, "(list)", "(list)")
	parseTestNoPath(t, "(list 1)", "(list 1)")
	parseTestNoPath(t, "(list 1 2)", "(list 1 2)")
	parseTestNoPath(t, "(list 1 2 (list))", "(list 1 2 (list))")
	parseTestNoPath(t, `(list 1 2 (list "a" "b" 3))`, `(list 1 2 (list "a" "b" 3))`)
	parseTestNoPath(t, `(list 1 2 (+ 1 2))`, `(list 1 2 3)`)

	parseTestNoPath(t, `(makeList 0 1)`, `(list)`)
	parseTestNoPath(t, `(makeList 1 1)`, `(list 1)`)
	parseTestNoPath(t, `(makeList 2 1)`, `(list 1 1)`)
	parseTestNoPath(t, `(makeList 3 (+ 1 1))`, `(list 2 2 2)`)
	parseTestNoPath(t, `(let ((a 2)) (makeList a 3))`, `(list 3 3)`)

	parseTestNoPath(t, "(cons 1 (list))", "(list 1)")
	parseTestNoPath(t, "(cons 1 (cons 2 (list)))", "(list 1 2)")

	parseTestNoPath(t, "(car (list 1))", "1")
	parseTestNoPath(t, "(car (list 1 2))", "1")

	parseTestNoPath(t, "(cdr (list 1))", "(list)")
	parseTestNoPath(t, "(cdr (list 1 2))", "(list 2)")
	parseTestNoPath(t, "(cdr (list 1 2 3))", "(list 2 3)")

	parseTestNoPath(t, "(cons 1 (cdr (list 1 2 3)))", "(list 1 2 3)")
	parseTestNoPath(t, "(car (cons 1 (cdr (list 1 2 3))))", "1")

	parseTestNoPath(t, `(nth 0 (list 1 2 3))`, "1")
	parseTestNoPath(t, `(nth 1 (list 1 2 3))`, "2")
	parseTestNoPath(t, `(nth 2 (list 1 2 3))`, "3")

	runtimeErrNoPath(t, `(nth (- 0 1) (list 1 2 3))`,
		"1: array index out of bounds: -1")
	runtimeErrNoPath(t, `(nth 5 (list 1 2 3))`,
		"1: array index out of bounds: 5")

	parseTestNoPath(t, `(map (lambda (x) (+ 1 x)) (list 1 2 3))`, "(list 2 3 4)")
	parseTestNoPath(t, `(map (lambda (x y) (+ x y)) (list 1 2 3) (list 4 5 6))`,
		"(list 5 7 9)")

	parseTestNoPath(t, `(map * (list 0 1 2) (list 3 4 5))`,
		"(list 0 4 10)")

	parseTestNoPath(t, `(map + (cons 1 (list 2)) (cons 3 (cons 4 (list))))`,
		"(list 4 6)")
}

func TestHashmap(t *testing.T) {
	parseTestNoPath(t, "(hashmap)", "(hashmap)")
	code := `(define a (hashmap))
(hashmapSet a "key" "value")`
	exp := `(define a (hashmap))
(hashmap ("key" "value"))`
	parseTestNoPath(t, code, exp)
	code = `(define a (hashmap))
(define b (hashmapSet a "key" "value"))
(hashmapGet b "key")`
	exp = `(define a (hashmap))
(define b (hashmap ("key" "value")))
"value"`
	parseTestNoPath(t, code, exp)
	code = `(define a (hashmap))
(hashmapGet (hashmapSet a "key" "value") "key")`
	exp = `(define a (hashmap))
"value"`
	parseTestNoPath(t, code, exp)
	code = `(define a (hashmap ("key1" "value1")))
(hashmapGet a "key1")`
	exp = `(define a (hashmap ("key1" "value1")))
"value1"`
	parseTestNoPath(t, code, exp)
	code = `(define a (hashmap ("key1" "value1") ("key2" "value2")))
(hashmapGet a "key1")
(hashmapGet a "key2")`
	exp = `(define a (hashmap ("key1" "value1") ("key2" "value2")))
"value1"
"value2"`
	parseTestNoPath(t, code, exp)
	code = `(define a (hashmap ("key1" "value1")))
(hashmapGet a "key1")
(define b (hashmapSet a "key2" "value2"))
(hashmapGet b "key1")
(hashmapGet b "key2")`
	exp = `(define a (hashmap ("key1" "value1")))
"value1"
(define b (hashmap ("key1" "value1") ("key2" "value2")))
"value1"
"value2"`
	parseTestNoPath(t, code, exp)
	code = `(define a (hashmap ("key1" "value1")))
(hashmapGet a "key1")
(define b (hashmapSet (hashmapSet a "key2" "value2") "key3" "value3"))
(hashmapGet b "key1")
(hashmapGet b "key2")
(hashmapGet b "key3")`
	exp = `(define a (hashmap ("key1" "value1")))
"value1"
(define b (hashmap ("key1" "value1") ("key2" "value2") ("key3" "value3")))
"value1"
"value2"
"value3"`
	parseTestNoPath(t, code, exp)
}

func TestDocker(t *testing.T) {
	checkContainers := func(code, expectedCode string, expected ...*Container) {
		ctx := parseTestNoPath(t, code, expectedCode)
		containerResult := Dsl{nil, ctx}.QueryContainers()
		if !reflect.DeepEqual(containerResult, expected) {
			t.Error(spew.Sprintf("test: %s, result: %s, expected: %s",
				code, containerResult, expected))
		}
	}

	code := `(docker "a")`
	checkContainers(code, code, &Container{Image: "a", Placement: Placement{make(map[[2]string]struct{})}})

	code = "(docker \"a\")\n(docker \"a\")"
	checkContainers(code, code, &Container{Image: "a", Placement: Placement{make(map[[2]string]struct{})}},
		&Container{Image: "a", Placement: Placement{make(map[[2]string]struct{})}})

	code = `(makeList 2 (list (docker "a") (docker "b")))`
	exp := `(list (list (docker "a") (docker "b"))` +
		` (list (docker "a") (docker "b")))`
	checkContainers(code, exp,
		&Container{Image: "a", Placement: Placement{make(map[[2]string]struct{})}},
		&Container{Image: "b", Placement: Placement{make(map[[2]string]struct{})}},
		&Container{Image: "a", Placement: Placement{make(map[[2]string]struct{})}},
		&Container{Image: "b", Placement: Placement{make(map[[2]string]struct{})}})
	code = `(list (docker "a" "c") (docker "b" "d" "e" "f"))`
	checkContainers(code, code,
		&Container{Image: "a", Command: []string{"c"}, Placement: Placement{make(map[[2]string]struct{})}},
		&Container{Image: "b", Command: []string{"d", "e", "f"}, Placement: Placement{make(map[[2]string]struct{})}})

	code = `(let ((a "foo") (b "bar")) (list (docker a) (docker b)))`
	exp = `(list (docker "foo") (docker "bar"))`
	checkContainers(code, exp, &Container{Image: "foo", Placement: Placement{make(map[[2]string]struct{})}},
		&Container{Image: "bar", Placement: Placement{make(map[[2]string]struct{})}})

	runtimeErrNoPath(t, `(docker bar)`, `1: unassigned variable: bar`)
	runtimeErrNoPath(t, `(docker 1)`, `1: docker arguments must be strings: 1`)
}

func TestMachines(t *testing.T) {
	checkMachines := func(code, expectedCode string, expected ...Machine) {
		ctx := parseTestNoPath(t, code, expectedCode)
		machineResult := Dsl{nil, ctx}.QueryMachineSlice("machines")
		if !reflect.DeepEqual(machineResult, expected) {
			t.Error(spew.Sprintf("test: %s, result: %v, expected: %v",
				code, machineResult, expected))
		}
	}

	// Test no attributes
	code := `(label "machines" (list (machine)))`
	expMachine := Machine{}
	expMachine.SetLabels([]string{"machines"})
	checkMachines(code, code, expMachine)

	// Test specifying the provider
	code = `(label "machines" (list (machine (provider "AmazonSpot"))))`
	expMachine = Machine{Provider: "AmazonSpot"}
	expMachine.SetLabels([]string{"machines"})
	checkMachines(code, code, expMachine)

	// Test making a list of machines
	code = `(label "machines" (makeList 2 (machine (provider "AmazonSpot"))))`
	expCode := `(label "machines" (list (machine (provider "AmazonSpot")) (machine (provider "AmazonSpot"))))`
	checkMachines(code, expCode, expMachine, expMachine)

	expMachine = Machine{Provider: "AmazonSpot", Size: "m4.large"}
	expMachine.SetLabels([]string{"machines"})
	code = `(label "machines" (list (machine (provider "AmazonSpot") (size "m4.large"))))`
	checkMachines(code, code, expMachine)

	// Test heterogenous sizes
	code = `(label "machines" (list (machine (provider "AmazonSpot") (size "m4.large")) (machine (provider "AmazonSpot") (size "m4.xlarge"))))`
	expMachine2 := Machine{Provider: "AmazonSpot", Size: "m4.xlarge"}
	expMachine2.SetLabels([]string{"machines"})
	checkMachines(code, code, expMachine, expMachine2)

	// Test heterogenous providers
	code = `(label "machines" (list (machine (provider "AmazonSpot") (size "m4.large")) (machine (provider "Vagrant"))))`
	expMachine2 = Machine{Provider: "Vagrant"}
	expMachine2.SetLabels([]string{"machines"})
	checkMachines(code, code, expMachine, expMachine2)

	// Test cpu range (two args)
	code = `(label "machines" (list (machine (provider "AmazonSpot") (cpu 4 8))))`
	expMachine = Machine{Provider: "AmazonSpot", CPU: Range{Min: 4, Max: 8}}
	expMachine.SetLabels([]string{"machines"})
	checkMachines(code, code, expMachine)

	// Test cpu range (one arg)
	code = `(label "machines" (list (machine (provider "AmazonSpot") (cpu 4))))`
	expMachine = Machine{Provider: "AmazonSpot", CPU: Range{Min: 4}}
	expMachine.SetLabels([]string{"machines"})
	checkMachines(code, code, expMachine)

	// Test ram range
	code = `(label "machines" (list (machine (provider "AmazonSpot") (ram 8 12))))`
	expMachine = Machine{Provider: "AmazonSpot", RAM: Range{Min: 8, Max: 12}}
	expMachine.SetLabels([]string{"machines"})
	checkMachines(code, code, expMachine)

	// Test float range
	code = `(label "machines" (list (machine (provider "AmazonSpot") (ram 0.5 2))))`
	expMachine = Machine{Provider: "AmazonSpot", RAM: Range{Min: 0.5, Max: 2}}
	expMachine.SetLabels([]string{"machines"})
	checkMachines(code, code, expMachine)

	// Test named attribute
	code = `(define large (list (ram 16) (cpu 8)))
(label "machines" (machine (provider "AmazonSpot") large))`
	expCode = `(define large (list (ram 16) (cpu 8)))
(label "machines" (machine (provider "AmazonSpot") (list (ram 16) (cpu 8))))`
	expMachine = Machine{Provider: "AmazonSpot", RAM: Range{Min: 16}, CPU: Range{Min: 8}}
	expMachine.SetLabels([]string{"machines"})
	checkMachines(code, expCode, expMachine)

	// Test invalid attribute type
	runtimeErrNoPath(t, `(machine (provider "AmazonSpot") "foo")`, `1: unrecognized argument to machine definition: "foo"`)
}

func TestMachineAttribute(t *testing.T) {
	checkMachines := func(code, expectedCode string, expected ...Machine) {
		ctx := parseTestNoPath(t, code, expectedCode)
		machineResult := Dsl{nil, ctx}.QueryMachineSlice("machines")
		if !reflect.DeepEqual(machineResult, expected) {
			t.Error(spew.Sprintf("test: %s, result: %v, expected: %v",
				code, machineResult, expected))
		}
	}

	// Test adding an attribute to an empty machine definition
	code := `(label "machines" (list (machine)))
(machineAttribute "machines" (provider "AmazonSpot"))`
	expMachine := Machine{Provider: "AmazonSpot"}
	expMachine.SetLabels([]string{"machines"})
	checkMachines(code, code, expMachine)

	// Test adding an attribute to a machine that already has another attribute
	code = `(label "machines" (list (machine (size "m4.large"))))
(machineAttribute "machines" (provider "AmazonSpot"))`
	expMachine = Machine{Provider: "AmazonSpot", Size: "m4.large"}
	expMachine.SetLabels([]string{"machines"})
	checkMachines(code, code, expMachine)

	// Test adding two attributes
	code = `(label "machines" (list (machine)))
(machineAttribute "machines" (provider "AmazonSpot") (size "m4.large"))`
	expMachine = Machine{Provider: "AmazonSpot", Size: "m4.large"}
	expMachine.SetLabels([]string{"machines"})
	checkMachines(code, code, expMachine)

	// Test replacing an attribute
	code = `(label "machines" (list (machine (provider "AmazonSpot") (size "m4.large"))))
(machineAttribute "machines" (size "m4.medium"))`
	expMachine = Machine{Provider: "AmazonSpot", Size: "m4.medium"}
	expMachine.SetLabels([]string{"machines"})
	checkMachines(code, code, expMachine)

	// Test setting attributes on a single machine (non-list)
	code = `(label "machines" (machine (provider "AmazonSpot")))
(machineAttribute "machines" (size "m4.medium"))`
	expMachine = Machine{Provider: "AmazonSpot", Size: "m4.medium"}
	expMachine.SetLabels([]string{"machines"})
	checkMachines(code, code, expMachine)

	// Test setting attributes on a bad label argument (non-string)
	code = `(machineAttribute 1 (machine (provider "AmazonSpot")))`
	runtimeErrNoPath(t, code, `1: machineAttribute key must be a string: 1`)

	// Test setting attributes on a non-existent label
	code = `(machineAttribute "badlabel" (machine (provider "AmazonSpot")))`
	runtimeErrNoPath(t, code, `1: machineAttribute key not defined: "badlabel"`)

	// Test setting attribute on a non-machine
	code = `(label "badlabel" (plaintextKey "key"))
(machineAttribute "badlabel" (machine (provider "AmazonSpot")))`
	badKey := plaintextKey{key: "key"}
	badKey.SetLabels([]string{"badlabel"})
	runtimeErrNoPath(t, code, fmt.Sprintf(`2: bad type, cannot change machine attributes: %s`, &badKey))

	// Test setting range attributes
	code = `(label "machines" (machine (provider "AmazonSpot")))
(machineAttribute "machines" (ram 1))`
	expMachine = Machine{Provider: "AmazonSpot", RAM: Range{Min: 1}}
	expMachine.SetLabels([]string{"machines"})
	checkMachines(code, code, expMachine)

	// Test setting using a named attribute
	code = `(define large (list (ram 16) (cpu 8)))
(label "machines" (machine (provider "AmazonSpot")))
(machineAttribute "machines" large)`
	expCode := `(define large (list (ram 16) (cpu 8)))
(label "machines" (machine (provider "AmazonSpot")))
(machineAttribute "machines" (list (ram 16) (cpu 8)))`
	expMachine = Machine{Provider: "AmazonSpot", RAM: Range{Min: 16}, CPU: Range{Min: 8}}
	expMachine.SetLabels([]string{"machines"})
	checkMachines(code, expCode, expMachine)
}

func TestKeys(t *testing.T) {
	getGithubKeys = func(username string) ([]string, error) {
		return []string{username}, nil
	}

	checkKeys := func(code, expectedCode string, expected ...string) {
		ctx := parseTestNoPath(t, code, expectedCode)
		keyResult := Dsl{nil, ctx}.QueryKeySlice("sshkeys")
		if !reflect.DeepEqual(keyResult, expected) {
			t.Error(spew.Sprintf("test: %s, result: %s, expected: %s",
				code, keyResult, expected))
		}
	}

	code := `(label "sshkeys" (list (plaintextKey "key")))`
	checkKeys(code, code, "key")

	code = `(label "sshkeys" (list (githubKey "user")))`
	checkKeys(code, code, "user")

	code = `(label "sshkeys" (list (githubKey "user") (plaintextKey "key")))`
	checkKeys(code, code, "user", "key")
}

func TestLabel(t *testing.T) {
	code := `(label "foo" (docker "a"))
(label "bar" "foo" (docker "b"))
(label "baz" "foo" "bar")
(label "baz2" "baz")
(label "qux" (docker "c"))`
	ctx := parseTestNoPath(t, code, code)

	containerA := &Container{Image: "a", Command: nil, Placement: Placement{make(map[[2]string]struct{})}}
	containerA.SetLabels([]string{"foo", "bar", "baz", "baz2"})
	containerB := &Container{Image: "b", Command: nil, Placement: Placement{make(map[[2]string]struct{})}}
	containerB.SetLabels([]string{"bar", "baz", "baz2"})
	containerC := &Container{Image: "c", Command: nil, Placement: Placement{make(map[[2]string]struct{})}}
	containerC.SetLabels([]string{"qux"})
	expected := []*Container{containerA, containerB, containerC}
	containerResult := Dsl{nil, ctx}.QueryContainers()
	if !reflect.DeepEqual(containerResult, expected) {
		t.Error(spew.Sprintf("\ntest: %s\nresult: %s\nexpected: %s",
			code, containerResult, expected))
	}

	code = `(label "foo" (makeList 2 (docker "a")))` +
		"\n(label \"bar\" \"foo\")"
	exp := `(label "foo" (list (docker "a") (docker "a")))` +
		"\n(label \"bar\" \"foo\")"
	ctx = parseTestNoPath(t, code, exp)
	expectedA := &Container{Image: "a", Command: nil, Placement: Placement{make(map[[2]string]struct{})}}
	expectedA.SetLabels([]string{"foo", "bar"})
	expected = []*Container{expectedA, expectedA}
	containerResult = Dsl{nil, ctx}.QueryContainers()
	if !reflect.DeepEqual(containerResult, expected) {
		t.Error(spew.Sprintf("\ntest: %s\nresult: %s\nexpected: %s",
			code, containerResult, expected))
	}

	runtimeErrNoPath(t, `(label 1 2)`, "1: label must be a string, found: 1")
	runtimeErrNoPath(t, `(label "foo" "bar")`, `1: undefined label: "bar"`)
	runtimeErrNoPath(t, `(label "foo" 1)`,
		"1: label must apply to atoms or other labels, found: 1")
	runtimeErrNoPath(t, `(label "foo" (docker "a")) (label "foo" "foo")`,
		"1: attempt to redefine label: foo")
}

func TestPlacement(t *testing.T) {
	// Normal
	code := `(label "red" (docker "a"))
(label "blue" (docker "b"))
(label "yellow" (docker "c"))
(placement "exclusive" "red" "blue" "yellow")`
	ctx := parseTestNoPath(t, code, code)
	containerA := Container{
		Image: "a", Placement: Placement{map[[2]string]struct{}{
			[2]string{"blue", "red"}:    {},
			[2]string{"blue", "yellow"}: {},
			[2]string{"red", "yellow"}:  {},
		}}}
	containerA.SetLabels([]string{"red"})
	containerB := Container{
		Image: "b", Placement: Placement{map[[2]string]struct{}{
			[2]string{"blue", "red"}:    {},
			[2]string{"blue", "yellow"}: {},
			[2]string{"red", "yellow"}:  {},
		}}}
	containerB.SetLabels([]string{"blue"})
	containerC := Container{
		Image: "c", Placement: Placement{map[[2]string]struct{}{
			[2]string{"blue", "red"}:    {},
			[2]string{"blue", "yellow"}: {},
			[2]string{"red", "yellow"}:  {},
		}}}
	containerC.SetLabels([]string{"yellow"})
	expected := []*Container{&containerA, &containerB, &containerC}
	containerResult := Dsl{nil, ctx}.QueryContainers()
	if !reflect.DeepEqual(containerResult, expected) {
		t.Error(spew.Sprintf("\ntest: %s\nresult  : %s\nexpected: %s",
			code, containerResult, expected))
	}

	// All on one
	code = `(label "red" (docker "a"))
(label "blue" "red")
(label "yellow" "red")
(placement "exclusive" "red" "blue" "yellow")`
	ctx = parseTestNoPath(t, code, code)
	containerA = Container{
		Image: "a", Placement: Placement{map[[2]string]struct{}{
			[2]string{"blue", "red"}:    {},
			[2]string{"blue", "yellow"}: {},
			[2]string{"red", "yellow"}:  {},
		}}}
	containerA.SetLabels([]string{"red", "blue", "yellow"})
	expected = []*Container{&containerA}
	containerResult = Dsl{nil, ctx}.QueryContainers()
	if !reflect.DeepEqual(containerResult, expected) {
		t.Error(spew.Sprintf("\ntest: %s\nresult  : %s\nexpected: %s",
			code, containerResult, expected))
	}

	// Duplicates
	code = `(label "red" (docker "a"))
(placement "exclusive" "red" "red" "red")`
	ctx = parseTestNoPath(t, code, code)
	containerA = Container{
		Image: "a", Placement: Placement{map[[2]string]struct{}{
			[2]string{"red", "red"}: {},
		}}}
	containerA.SetLabels([]string{"red"})
	expected = []*Container{&containerA}
	containerResult = Dsl{nil, ctx}.QueryContainers()
	if !reflect.DeepEqual(containerResult, expected) {
		t.Error(spew.Sprintf("\ntest: %s\nresult  : %s\nexpected: %s",
			code, containerResult, expected))
	}

	// Unrelated definitions
	code = `(label "red" (docker "a"))
(placement "exclusive" "red" "red")
(label "blue" (docker "b"))
(placement "exclusive" "blue" "blue")`
	ctx = parseTestNoPath(t, code, code)
	containerA = Container{
		Image: "a", Placement: Placement{map[[2]string]struct{}{
			[2]string{"red", "red"}: {},
		}}}
	containerA.SetLabels([]string{"red"})
	containerB = Container{
		Image: "b", Placement: Placement{map[[2]string]struct{}{
			[2]string{"blue", "blue"}: {},
		}}}
	containerB.SetLabels([]string{"blue"})
	expected = []*Container{&containerA, &containerB}
	containerResult = Dsl{nil, ctx}.QueryContainers()
	if !reflect.DeepEqual(containerResult, expected) {
		t.Error(spew.Sprintf("\ntest: %s\nresult  : %s\nexpected: %s",
			code, containerResult, expected))
	}
}

func TestConnect(t *testing.T) {
	code := `(label "a" (docker "alpine"))
(label "b" (docker "alpine"))
(label "c" (docker "alpine"))
(label "d" (docker "alpine"))
(label "e" (docker "alpine"))
(label "f" (docker "alpine"))
(label "g" (docker "alpine"))
(connect 80 "a" "b")
(connect 80 "a" "b" "c")
(connect (list 1 65534) "b" "c")
(connect (list 0 65535) "a" "c")
(connect 443 "c" "d" "e" "f")
(connect (list 100 65535) "g" "g")`
	ctx := parseTestNoPath(t, code, code)

	expected := map[Connection]struct{}{
		{"a", "b", 80, 80}:     {},
		{"a", "c", 80, 80}:     {},
		{"b", "c", 1, 65534}:   {},
		{"a", "c", 0, 65535}:   {},
		{"c", "d", 443, 443}:   {},
		{"c", "e", 443, 443}:   {},
		{"c", "f", 443, 443}:   {},
		{"g", "g", 100, 65535}: {},
	}

	for exp := range expected {
		if _, ok := ctx.connections[exp]; !ok {
			t.Error(spew.Sprintf("Missing connection: %v", exp))
			continue
		}

		delete(ctx.connections, exp)
	}

	if len(ctx.connections) > 0 {
		t.Error(spew.Sprintf("Unexpected connections: %v", ctx.connections))
	}

	runtimeErrNoPath(t, `(connect a "foo" "bar")`, "1: unassigned variable: a")
	runtimeErrNoPath(t, `(connect (list 80) "foo" "bar")`,
		"1: port range must have two ints: (list 80)")
	runtimeErrNoPath(t, `(connect (list 0 70000) "foo" "bar")`,
		"1: invalid port range: [0, 70000]")
	runtimeErrNoPath(t, `(connect (list (- 0 10) 10) "foo" "bar")`,
		"1: invalid port range: [-10, 10]")
	runtimeErrNoPath(t, `(connect (list 100 10) "foo" "bar")`,
		"1: invalid port range: [100, 10]")
	runtimeErrNoPath(t, `(connect "80" "foo" "bar")`,
		"1: port range must be an int or a list of ints: \"80\"")
	runtimeErrNoPath(t, `(connect (list "a" "b") "foo" "bar")`,
		"1: port range must have two ints: (list \"a\" \"b\")")
	runtimeErrNoPath(t, `(connect 80 4 5)`, "1: connect applies to labels: 4")
	runtimeErrNoPath(t, `(connect 80 "foo" "foo")`, "1: connect undefined label: \"foo\"")
}

func TestImport(t *testing.T) {
	// Test module keyword
	parseTestNoPath(t, `(module "math" (define Square (lambda (x) (* x x))))
(math.Square 2)`,
		`(module "math" (define Square (lambda (x) (* x x))))
4`)

	// Test module with multiple-statement body
	parseTestNoPath(t, `(module "math" (define three 3) (define Triple (lambda (x) (* three x))))
(math.Triple 2)`,
		`(module "math" (define three 3)
(define Triple (lambda (x) (* three x))))
6`)

	// Test importing from disk
	testFs := afero.NewMemMapFs()
	util.AppFs = testFs
	util.WriteFile("math.spec", []byte("(define Square (lambda (x) (* x x)))"), 0644)
	util.AppFs = testFs
	parseTest(t, `(import "math")
(math.Square 2)`, `(module "math" (define Square (lambda (x) (* x x))))
4`, []string{"."})

	// Test two imports in separate directories
	testFs = afero.NewMemMapFs()
	util.AppFs = testFs
	testFs.Mkdir("square", 777)
	util.WriteFile("square/square.spec", []byte("(define Square (lambda (x) (* x x)))"), 0644)
	testFs.Mkdir("cube", 777)
	util.WriteFile("cube/cube.spec", []byte("(define Cube (lambda (x) (* x x x)))"), 0644)
	parseTest(t, `(import "square")
(import "cube")
(square.Square 2)
(cube.Cube 2)`, `(module "square" (define Square (lambda (x) (* x x))))
(module "cube" (define Cube (lambda (x) (* x x x))))
4
8`, []string{"square", "cube"})

	// Test import with an import
	testFs = afero.NewMemMapFs()
	util.AppFs = testFs
	util.WriteFile("square.spec", []byte("(define Square (lambda (x) (* x x)))"), 0644)
	util.WriteFile("cube.spec", []byte(`(import "square")
(define Cube (lambda (x) (* x (square.Square x))))`), 0644)
	parseTest(t, `(import "cube")
(cube.Cube 2)`, `(module "cube" (module "square" (define Square (lambda (x) (* x x))))
(define Cube (lambda (x) (* x (square.Square x)))))
8`, []string{"."})

	// Test error in an imported module
	testFs = afero.NewMemMapFs()
	util.AppFs = testFs
	util.WriteFile("bad.spec", []byte(`(define BadFunc (lambda () (+ 1 "1")))`), 0644)
	runtimeErr(t, `(import "bad")
(bad.BadFunc)`, `./bad.spec:1: bad arithmetic argument: "1"`, []string{"."})

	testFs = afero.NewMemMapFs()
	util.AppFs = testFs
	util.WriteFile("A.spec", []byte(`(import "A")`), 0644)
	runtimeErr(t, `(import "A")`, `./A.spec:1: import cycle: [A A]`, []string{"."})

	testFs = afero.NewMemMapFs()
	util.AppFs = testFs
	util.WriteFile("A.spec", []byte(`(import "B")`), 0644)
	util.WriteFile("B.spec", []byte(`(import "A")`), 0644)
	runtimeErr(t, `(import "A")`, `./B.spec:1: import cycle: [A B A]`, []string{"."})

	testFs = afero.NewMemMapFs()
	util.AppFs = testFs
	util.WriteFile("A.spec", []byte(`(import "B")
(import "C")
(define AddTwo (lambda (x) (+ (B.AddOne x) C.One)))`), 0644)
	util.WriteFile("B.spec", []byte(`(import "C")
(define AddOne (lambda (x) (+ x C.One)))`), 0644)
	util.WriteFile("C.spec", []byte(`(define One 1)`), 0644)
	parseTest(t, `(import "A")
(A.AddTwo 1)`, `(module "A" (module "B" (module "C" (define One 1))
(define AddOne (lambda (x) (+ x C.One))))
(module "C" (define One 1))
(define AddTwo (lambda (x) (+ (B.AddOne x) C.One))))
3`, []string{"."})

	// Test that non-capitalized binds are not exported
	runtimeErrNoPath(t, `(module "A" (define addOne (lambda (x) (+ x 1))))
(A.addOne 1)`, `2: unknown function: A.addOne`)

	// Test that non-capitalized labels are not exported
	runtimeErrNoPath(t, `(module "A" (label "a-container" (docker "A")))
(connect 80 "A.a-container" (docker "B"))`, `2: connect undefined label: "A.a-container"`)

	// Test that capitalized labels are properly exported
	code := `(module "keys" (label "Grads" (plaintextKey "ejj")))`
	ctx := parseTestNoPath(t, code, code)
	keyResult := Dsl{nil, ctx}.QueryKeySlice("keys.Grads")
	if len(keyResult) != 1 || keyResult[0] != "ejj" {
		t.Error(spew.Sprintf("test: %s, result: %s, expected: %s",
			code, keyResult, "ejj"))
	}
}

func TestScanError(t *testing.T) {
	parseErr(t, "\"foo", "literal not terminated")
}

func TestParseErrors(t *testing.T) {
	unbalanced := "1: unbalanced Parenthesis"
	parseErr(t, "(", unbalanced)
	parseErr(t, ")", unbalanced)
	parseErr(t, "())", unbalanced)
	parseErr(t, "(((())", unbalanced)
	parseErr(t, "((+ 5 (* 3 7)))))", unbalanced)
}

func TestRuntimeErrors(t *testing.T) {
	err := `1: bad arithmetic argument: "a"`
	runtimeErrNoPath(t, `(+ "a" "a")`, err)
	runtimeErrNoPath(t, `(list (+ "a" "a"))`, err)
	runtimeErrNoPath(t, `(let ((y (+ "a" "a"))) y)`, err)
	runtimeErrNoPath(t, `(let ((y 3)) (+ "a" "a"))`, err)

	runtimeErrNoPath(t, "(define a 3) (define a 3)", `1: attempt to redefine: "a"`)
	runtimeErrNoPath(t, "(define a (+ 3 b ))", "1: unassigned variable: b")

	runtimeErrNoPath(t, `(makeList a 3)`, "1: unassigned variable: a")
	runtimeErrNoPath(t, `(makeList 3 a)`, "1: unassigned variable: a")
	runtimeErrNoPath(t, `(makeList "a" 3)`,
		`1: makeList must begin with a positive integer, found: "a"`)

	runtimeErrNoPath(t, `(label a a)`, "1: unassigned variable: a")

	runtimeErrNoPath(t, "(1 2 3)", "1: S-expressions must start with a function call: 1")

	args := "1: not enough arguments: +"
	runtimeErrNoPath(t, "(+)", args)
	runtimeErrNoPath(t, "(+ 5)", args)
	runtimeErrNoPath(t, "(+ 5 (+ 6))", args)

	runtimeErrNoPath(t, "()", "1: S-expressions must start with a function call: ()")
	runtimeErrNoPath(t, "(let)", "1: not enough arguments: let")
	runtimeErrNoPath(t, "(let 3 a)", "1: let binds must be defined in an S-expression")
	runtimeErrNoPath(t, "(let (a) a)", "1: binds must be exactly 2 arguments: a")
	runtimeErrNoPath(t, "(let ((a)) a)", "1: binds must be exactly 2 arguments: (a)")
	runtimeErrNoPath(t, "(let ((3 a)) a)", "1: bind name must be an ident: 3")
	runtimeErrNoPath(t, "(let ((a (+))) a)", args)
	runtimeErrNoPath(t, "(let ((a 3)) (+))", args)

	runtimeErrNoPath(t, "(define a (+))", args)

	runtimeErrNoPath(t, "(badFun)", "1: unknown function: badFun")
}

func TestErrorMetadata(t *testing.T) {
	var checkParseErr = func(path string, expErr string) {
		sc := scanner.Scanner{
			Position: scanner.Position{
				Filename: path,
			},
		}
		f, err := util.Open(path)
		if err != nil {
			t.Errorf("Couldn't open %s", path)
		}
		_, err = parse(*sc.Init(f))
		if err.Error() != expErr {
			t.Errorf("Expected \"%s\"\ngot \"%s\"", expErr, err)
			return
		}
	}
	var checkEvalErr = func(path string, expErr string) {
		sc := scanner.Scanner{
			Position: scanner.Position{
				Filename: path,
			},
		}
		f, err := util.Open(path)
		if err != nil {
			t.Errorf("Couldn't open %s", path)
		}
		parsed, err := parse(*sc.Init(f))
		if err != nil {
			t.Errorf("Unexpected parse error: %s", parsed)
		}

		_, _, err = eval(parsed, []string{"."})
		if err.Error() != expErr {
			t.Errorf("Expected \"%s\"\ngot \"%s\"", expErr, err)
			return
		}
	}

	util.AppFs = afero.NewMemMapFs()
	util.WriteFile("paren.spec", []byte(`
// This is a comment.
(define Test "abc"`), 0644)
	checkParseErr("paren.spec", "paren.spec:3: unbalanced Parenthesis")

	util.WriteFile("undefined.spec", []byte(`
(+ 1 b)`), 0644)
	checkEvalErr("undefined.spec", "undefined.spec:2: unassigned variable: b")

	util.WriteFile("bad_type.spec", []byte(`
(define a "1")
(+ 1 a)`), 0644)
	checkEvalErr("bad_type.spec", `bad_type.spec:3: bad arithmetic argument: "1"`)
}

func TestQuery(t *testing.T) {
	var sc scanner.Scanner
	dsl, err := New(*sc.Init(strings.NewReader("(")), []string{})
	if err == nil {
		t.Error("Expected error")
	}

	dsl, err = New(*sc.Init(strings.NewReader("(+ a a)")), []string{})
	if err == nil {
		t.Error("Expected runtime error")
	}

	dsl, err = New(*sc.Init(strings.NewReader(`
		(define a (+ 1 2))
		(define b "This is b")
		(define c (list "This" "is" "b"))
		(define d (list "1" 2 "3"))
		(define e 1.5)
		(label "sshkeys" (list (plaintextKey "key") (githubKey "github")))
		(docker b)
		(docker b)`)), []string{})
	if err != nil {
		t.Error(err)
		return
	}

	if val := dsl.QueryInt("a"); val != 3 {
		t.Error(val, "!=", 3)
	}

	if val := dsl.QueryInt("missing"); val != 0 {
		t.Error(val, "!=", 3)
	}

	if val := dsl.QueryInt("b"); val != 0 {
		t.Error(val, "!=", 3)
	}

	if val, _ := dsl.QueryFloat("e"); val != 1.5 {
		t.Error(val, "!=", 1.5)
	}

	if val := dsl.QueryString("b"); val != "This is b" {
		t.Error(val, "!=", "This is b")
	}

	if val := dsl.QueryString("missing"); val != "" {
		t.Error(val, "!=", "")
	}

	if val := dsl.QueryString("a"); val != "" {
		t.Error(val, "!=", "")
	}

	expected := []string{"This", "is", "b"}
	if val := dsl.QueryStrSlice("c"); !reflect.DeepEqual(val, expected) {
		t.Error(val, "!=", expected)
	}

	if val := dsl.QueryStrSlice("d"); val != nil {
		t.Error(val, "!=", nil)
	}

	if val := dsl.QueryStrSlice("missing"); val != nil {
		t.Error(val, "!=", nil)
	}

	if val := dsl.QueryStrSlice("a"); val != nil {
		t.Error(val, "!=", nil)
	}

	if val := dsl.QueryContainers(); len(val) != 2 {
		t.Error(val)
	}

	if val := dsl.QueryKeySlice("sshkeys"); len(val) != 2 {
		t.Error(val)
	}
}

func parseTestNoPath(t *testing.T, code, evalExpected string) evalCtx {
	return parseTest(t, code, evalExpected, []string{})
}

func parseTest(t *testing.T, code, evalExpected string, path []string) evalCtx {
	var sc scanner.Scanner
	parsed, err := parse(*sc.Init(strings.NewReader(code)))
	if err != nil {
		t.Errorf("%s: %s", code, err)
		return evalCtx{}
	}

	if str := parsed.String(); str != code {
		t.Errorf("\nParse expected \"%s\"\ngot \"%s\"", code, str)
		return evalCtx{}
	}

	result, ctx, err := eval(parsed, path)
	if err != nil {
		t.Errorf("%s: %s", code, err)
		return evalCtx{}
	}

	if result.String() != evalExpected {
		t.Errorf("\nEval expected \"%s\"\ngot \"%s\"", evalExpected, result)
		return evalCtx{}
	}

	// The code may be re-evaluated by the minions.  If that happens, the result
	// should be exactly the same.
	eval2, _, err := eval(result, path)
	if err != nil {
		t.Errorf("%s: %s", code, err)
		return evalCtx{}
	}

	if eval2.String() != result.String() {
		t.Errorf("\nEval expected \"%s\"\ngot \"%s\"", result, eval2)
		return evalCtx{}
	}

	return ctx
}

func parseErr(t *testing.T, code, expectedErr string) {
	var sc scanner.Scanner
	_, err := parse(*sc.Init(strings.NewReader(code)))
	if fmt.Sprintf("%s", err) != expectedErr {
		t.Errorf("%s: %s", code, err)
	}
}

func runtimeErrNoPath(t *testing.T, code, expectedErr string) {
	runtimeErr(t, code, expectedErr, []string{})
}

func runtimeErr(t *testing.T, code, expectedErr string, path []string) {
	var sc scanner.Scanner
	prog, err := parse(*sc.Init(strings.NewReader(code)))
	if err != nil {
		t.Errorf("%s: %s", code, err)
		return
	}

	_, _, err = eval(prog, path)
	if fmt.Sprintf("%s", err) != expectedErr {
		t.Errorf("%s: %s", code, err)
		return
	}
}
