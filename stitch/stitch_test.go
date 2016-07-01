package stitch

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"text/scanner"

	"github.com/NetSys/quilt/util"

	"github.com/davecgh/go-spew/spew"
	"github.com/spf13/afero"
)

func TestRange(t *testing.T) {
	parseTest(t, "(range 0)", "(list)")
	parseTest(t, "(range 1)", "(list 0)")
	parseTest(t, "(range 2)", "(list 0 1)")

	parseTest(t, "(range 1 1)", "(list)")
	parseTest(t, "(range 1 2)", "(list 1)")
	parseTest(t, "(range 1 3)", "(list 1 2)")

	parseTest(t, "(range 0 5 2)", "(list 0 2 4)")
	parseTest(t, "(range 0 1 2)", "(list 0)")

	parseTest(t, "(map (lambda (x) (+ 1 x)) (range 3))", "(list 1 2 3)")
}

func TestArith(t *testing.T) {
	parseTest(t, "2", "2")

	parseTest(t, "(+ 5 3)", "8")
	parseTest(t, "(+ 5 3 10)", "18")

	parseTest(t, "(- 100 50)", "50")
	parseTest(t, "(- 100 50 100)", "-50")

	parseTest(t, "(* 2 6)", "12")
	parseTest(t, "(* 2 6 3)", "36")

	parseTest(t, "(/ 100 10)", "10")
	parseTest(t, "(/ 100 10 5)", "2")

	parseTest(t, "(% 10 100)", "10")
	parseTest(t, "(% 10 100 3)", "1")

	parseTest(t, "(+ (* 3 (- 10 2)) (/ 100 (* 25 2)))", "26")
}

func TestStrings(t *testing.T) {
	code := `"foo"`
	parseTest(t, code, code)

	code = "\"foo\"\n\"bar\""
	parseTest(t, code, code)

	code = "\"foo\"\n5\n\"bar\""
	parseTest(t, code, code)

	code = `(sprintf "foo")`
	parseTest(t, code, `"foo"`)

	code = `(sprintf "%s %s" "foo" "bar")`
	parseTest(t, code, `"foo bar"`)

	code = `(sprintf "%s %d" "foo" 3)`
	parseTest(t, code, `"foo 3"`)

	code = `(sprintf "%s %s" "foo" (list 1 2 3))`
	parseTest(t, code, `"foo (list 1 2 3)"`)

	parseTest(t, `(+ "foo" "bar")`, `"foobar"`)
	parseTest(t, `(+ "foo" "")`, `"foo"`)
	parseTest(t, `(+ "foo" "" "bar" "baz")`, `"foobarbaz"`)

	runtimeErr(t, "(sprintf a)", "1: unassigned variable: a")
	runtimeErr(t, "(sprintf 1)", "1: sprintf format must be a string: 1")
}

func TestLet(t *testing.T) {
	parseTest(t, "(let ((a 5)) a)", "5")
	parseTest(t, "(+ 12 (let ((a 5)) a))", "17")
	parseTest(t, "(let ((a 5)) (* a a))", "25")
	parseTest(t, "(let ((a 5) (b 2)) (* a b))", "10")
	parseTest(t, "(let ((a 2)) (* (let ((a 3)) (* a a)) (* a a)))", "36")

	// Test the implicit progn
	parseTest(t, "(let ((a 2)) (define b 3) (* a b))", "6")

	// Test let*-ness
	parseTest(t, "(let ((a 1) (b (+ a a)) (c (+ a b)) (d (* b c))) (+ d d))", "12")

	// Test let with an omitted body
	parseTest(t, "(let ((a 1)))", "(list)")
}

func TestSet(t *testing.T) {
	parseTest(t, "(progn (define a 3) (set a 4) a)", "4")

	runtimeErr(t, "(set a 4)", "1: undefined varaible: a")
}

func TestApply(t *testing.T) {
	parseTest(t, "(apply + (list 3 5))", "8")
	parseTest(t,
		"(apply (lambda (x y) (+ x y)) (let ((x 3)) (list x 5)))", "8")
}

func TestLambda(t *testing.T) {
	// Single argument lambda
	parseTest(t, "((lambda (x) (* x x)) 5)", "25")

	// Two argument lambda
	parseTest(t, "((lambda (x y) (* x y)) 5 6)", "30")

	parseTest(t, "((lambda (x y) (* 2 (+ x y))) 5 6)", "22")

	// Unevaluated argument
	parseTest(t, "((lambda (x y) (+ x y)) 5 (* 2 6))", "17")

	// Named lambda
	parseTest(t, "(progn (define Square (lambda (x) (* x x))) (Square 5))", "25")

	// Named lambda in let
	parseTest(t, "(let ((Square (lambda (x) (* x x)))) (Square 6))", "36")

	// Two named lambdas
	cubeDef := "(define Square (lambda (x) (* x x)))\n" +
		"(define Cube (lambda (x) (* x (Square x))))\n"
	parseTest(t, fmt.Sprintf("(progn %s %s", cubeDef, "(Cube 5))"), "125")

	// Test closure
	adder := "(progn " +
		"(define nAdder (lambda (n) (lambda (x) (+ n x))))\n" +
		"(define fiveAdder (nAdder 5))\n" +
		"(fiveAdder 10))"
	parseTest(t, adder, "15")

	// Test variable masking
	adder = "((let ((x 5)) (lambda (x) (+ x 1))) 1)"
	parseTest(t, adder, "2")

	// Test Implicit Progn
	parseTest(t, `((lambda (x y z) x y z) 1 2 3)`, "3")

	// Test define function syntax
	parseTest(t, "(progn (define (Square x) (* x x)) (Square 4))", "16")
	parseTest(t, "(progn (define (Five) 5) (Five))", "5")
	parseTest(t, "(progn (define (PlusPlus x y z) (+ x y z)) (PlusPlus 1 2 3))", "6")

	// Test that recursion DOESN'T work
	fib := "(define fib (lambda (n) (if (= n 0) 1 (* n (fib (- n 1)))))) (fib 5)"
	runtimeErr(t, fib, "1: unknown function: fib")

	// Test body-less lambda
	runtimeErr(t, "(lambda (x))", "1: not enough arguments: lambda")
	runtimeErr(t, "(lambda)", "1: not enough arguments: lambda")
}

func TestProgn(t *testing.T) {
	// Test that first argument gets run
	runtimeErr(t, `(progn (+ 1 "1") (+ 2 2))`, `1: bad arithmetic argument: "1"`)

	// Test that only the second arguments gets returned
	parseTest(t, "(progn (+ 1 1) (+ 2 2))", "4")
}

func TestIf(t *testing.T) {
	// Test boolean atoms
	trueTest := "(if true 1 0)"
	parseTest(t, trueTest, "1")

	falseTest := "(if false 1 0)"
	parseTest(t, falseTest, "0")

	// Test no else case
	falseTest = "(if false 1)"
	parseTest(t, falseTest, "(list)")

	// Test one-argument and
	trueTest = "(if (and true) 1 0)"
	parseTest(t, trueTest, "1")

	falseTest = "(if (and false) 1 0)"
	parseTest(t, falseTest, "0")

	// Test one-argument or
	trueTest = "(if (or true) 1 0)"
	parseTest(t, trueTest, "1")

	falseTest = "(if (or false) 1 0)"
	parseTest(t, falseTest, "0")

	// Test two-argument and
	trueTest = "(if (and true true) 1 0)"
	parseTest(t, trueTest, "1")

	falseTest = "(if (and true false) 1 0)"
	parseTest(t, falseTest, "0")

	// Test two-argument or
	trueTest = "(if (or false true) 1 0)"
	parseTest(t, trueTest, "1")

	falseTest = "(if (or false false) 1 0)"
	parseTest(t, falseTest, "0")

	// Test short-circuiting
	andShort := "(if (and false (/ 1 0)) 1 0)"
	parseTest(t, andShort, "0")

	orShort := "(if (or true (/ 1 0)) 1 0)"
	parseTest(t, orShort, "1")

	// Test = on ints
	compTest := "(= 1 1)"
	parseTest(t, compTest, "true")

	compTest = "(= 1 2)"
	parseTest(t, compTest, "false")

	// Test = on strings
	compTest = `(= "hello" "hello")`
	parseTest(t, compTest, "true")

	compTest = `(= "hello" "world")`
	parseTest(t, compTest, "false")

	// Test = on lists
	compTest = `(= (list "hello" "world") (list "hello" "world"))`
	parseTest(t, compTest, "true")

	compTest = `(= (list "hello" "world") (list "world hello"))`
	parseTest(t, compTest, "false")

	// Test <
	compTest = "(< 2 1)"
	parseTest(t, compTest, "false")

	compTest = "(< 1 2)"
	parseTest(t, compTest, "true")

	// Test >
	compTest = "(> 1 2)"
	parseTest(t, compTest, "false")

	compTest = "(> 2 1)"
	parseTest(t, compTest, "true")

	// Test !
	notTest := "(! false)"
	parseTest(t, notTest, "true")

	notTest = "(! true)"
	parseTest(t, notTest, "false")

	// Test that an empty list is false
	parseTest(t, "(bool (list))", "false")

	// Test that an empty string is false
	parseTest(t, `(bool "")`, "false")

	// Test that 0 is false
	parseTest(t, "(bool 0)", "false")

	// Test that a non-bool gets properly converted
	parseTest(t, `(bool (docker "foo"))`, "true")
}

func TestDefine(t *testing.T) {
	parseTest(t, "(progn (define a 1) a)", "1")

	parseTest(t, "(progn (define a 1) (define b 2) (list a b))", "(list 1 2)")

	parseTest(t, "(list (define a 1) 3 (define b 2) a b)",
		"(list (list) 3 (list) 1 2)")

	parseTest(t, "(list (define a (+ 5 7)) a)", "(list (list) 12)")

	parseTest(t, "(progn (define a (+ 1 1)) (define b (* a 2)) (list a b))",
		"(list 2 4)")
}

func TestList(t *testing.T) {
	parseTest(t, "(list)", "(list)")
	parseTest(t, "(list 1)", "(list 1)")
	parseTest(t, "(list 1 2)", "(list 1 2)")
	parseTest(t, "(list 1 2 (list))", "(list 1 2 (list))")
	parseTest(t, `(list 1 2 (list "a" "b" 3))`, `(list 1 2 (list "a" "b" 3))`)
	parseTest(t, `(list 1 2 (+ 1 2))`, `(list 1 2 3)`)

	parseTest(t, `(makeList 0 1)`, `(list)`)
	parseTest(t, `(makeList 1 1)`, `(list 1)`)
	parseTest(t, `(makeList 2 1)`, `(list 1 1)`)
	parseTest(t, `(makeList 3 (+ 1 1))`, `(list 2 2 2)`)
	parseTest(t, `(let ((a 2)) (makeList a 3))`, `(list 3 3)`)

	parseTest(t, `(len (list))`, `0`)
	parseTest(t, `(len (list 1))`, `1`)

	parseTest(t, "(cons 1 (list))", "(list 1)")
	parseTest(t, "(cons 1 (cons 2 (list)))", "(list 1 2)")

	parseTest(t, "(car (list 1))", "1")
	parseTest(t, "(car (list 1 2))", "1")

	parseTest(t, "(cdr (list 1))", "(list)")
	parseTest(t, "(cdr (list 1 2))", "(list 2)")
	parseTest(t, "(cdr (list 1 2 3))", "(list 2 3)")

	parseTest(t, "(cons 1 (cdr (list 1 2 3)))", "(list 1 2 3)")
	parseTest(t, "(car (cons 1 (cdr (list 1 2 3))))", "1")

	parseTest(t, "(append (list) 1)", "(list 1)")
	parseTest(t, "(append (list 1) 2)", "(list 1 2)")
	parseTest(t, "(append (list 1) 2 3)", "(list 1 2 3)")
	runtimeErr(t, "(append 1 1)", "1: append applies to lists: 1")

	parseTest(t, `(nth 0 (list 1 2 3))`, "1")
	parseTest(t, `(nth 1 (list 1 2 3))`, "2")
	parseTest(t, `(nth 2 (list 1 2 3))`, "3")

	runtimeErr(t, `(nth (- 0 1) (list 1 2 3))`,
		"1: array index out of bounds: -1")
	runtimeErr(t, `(nth 5 (list 1 2 3))`,
		"1: array index out of bounds: 5")

	parseTest(t, `(map (lambda (x) (+ 1 x)) (list 1 2 3))`, "(list 2 3 4)")
	parseTest(t, `(map (lambda (x y) (+ x y)) (list 1 2 3) (list 4 5 6))`,
		"(list 5 7 9)")

	parseTest(t, `(map * (list 0 1 2) (list 3 4 5))`,
		"(list 0 4 10)")

	parseTest(t, `(map + (cons 1 (list 2)) (cons 3 (cons 4 (list))))`,
		"(list 4 6)")

	parseTest(t, `(reduce * (list 1 2 3 4))`, "24")
	parseTest(t, `(reduce (lambda (x y) (+ (* x 10) y)) (list 1 2 3 4))`, "1234")
	parseTest(t, `(reduce (lambda (x y) (sprintf "%s,%s" x y)) (list "a" "b" "c"))`,
		`"a,b,c"`)
	runtimeErr(t, `(reduce * (list 1))`, "1: not enough elements to reduce: (list 1)")
}

func TestHmap(t *testing.T) {
	parseTest(t, "(hmap)", "(hmap)")

	code := `(define a (hmap)) (hmapSet a "key" "value")`
	exp := `(list) (hmap ("key" "value"))`
	parseTest(t, code, exp)

	code = `(define a (hmap)) (define b (hmapSet a "key" "value")) (hmapGet b "key")`
	exp = `(list) (list) "value"`
	parseTest(t, code, exp)

	code = `(define a (hmap)) (hmapGet (hmapSet a "key" "value") "key")`
	exp = `(list) "value"`
	parseTest(t, code, exp)

	code = `(define a (hmap ("key1" "value1"))) (hmapGet a "key1")`
	exp = `(list) "value1"`
	parseTest(t, code, exp)

	code = `(define a (hmap ("key1" "value1") ("key2" "value2")))
	(hmapGet a "key1")
	(hmapGet a "key2")`
	exp = `(list) "value1" "value2"`
	parseTest(t, code, exp)

	code = `(define a (hmap ("key1" "value1")))
	(hmapGet a "key1")
	(define b (hmapSet a "key2" "value2"))
	(hmapGet b "key1")
	(hmapGet b "key2")`
	exp = `(list) "value1" (list) "value1" "value2"`
	parseTest(t, code, exp)

	code = `(define a (hmap ("key1" "value1")))
	(hmapGet a "key1")
	(define b (hmapSet (hmapSet a "key2" "value2") "key3" "value3"))
	(hmapGet b "key1")
	(hmapGet b "key2")
	(hmapGet b "key3")`
	exp = `(list) "value1" (list) "value1" "value2" "value3"`
	parseTest(t, code, exp)

	code = `(define a (hmap ("key1" "value1")))
	(hmapContains a "key1")
	(hmapContains a "key2")`
	exp = `(list) true false`
	parseTest(t, code, exp)

	code = `(define a (hmap ("key1" "value1")))
	(hmapKeys a)
	(hmapValues a)`
	exp = `(list) (list "key1") (list "value1")`
	parseTest(t, code, exp)
}

func TestDocker(t *testing.T) {
	checkContainers := func(code, expectedCode string, expected ...*Container) {
		ctx := parseTest(t, code, expectedCode)
		containerResult := Stitch{"", ctx}.QueryContainers()
		if !reflect.DeepEqual(containerResult, expected) {
			t.Error(spew.Sprintf("test: %v, result: %v, expected: %v",
				code, containerResult, expected))
		}
	}

	env := map[string]string{}
	code := `(docker "a")`
	checkContainers(code, code, &Container{ID: 1, Image: "a", Env: env})

	code = "(docker \"a\")\n(docker \"a\")"
	checkContainers(code, code, &Container{ID: 1, Image: "a", Env: env},
		&Container{ID: 2, Image: "a", Env: env})

	code = `(makeList 2 (list (docker "a") (docker "b")))`
	exp := `(list (list (docker "a") (docker "b"))` +
		` (list (docker "a") (docker "b")))`
	checkContainers(code, exp,
		&Container{ID: 1, Image: "a", Env: env},
		&Container{ID: 2, Image: "b", Env: env},
		&Container{ID: 3, Image: "a", Env: env},
		&Container{ID: 4, Image: "b", Env: env})
	code = `(list (docker "a" "c") (docker "b" (list "d" "e" "f")))`
	exp = `(list (docker "a" "c") (docker "b" "d" "e" "f"))`
	checkContainers(code, exp,
		&Container{ID: 1, Image: "a", Command: []string{"c"}, Env: env},
		&Container{ID: 2, Image: "b", Command: []string{"d", "e", "f"},
			Env: env})

	code = `(let ((a "foo") (b "bar")) (list (docker a) (docker b)))`
	exp = `(list (docker "foo") (docker "bar"))`
	checkContainers(code, exp, &Container{ID: 1, Image: "foo", Env: env},
		&Container{ID: 2, Image: "bar", Env: env})

	// Test creating containers from within a lambda function
	code = `((lambda () (docker "foo")))`
	exp = `(docker "foo")`
	checkContainers(code, exp, &Container{ID: 1, Image: "foo", Env: env})

	code = `(define (make) (docker "a") (docker "b") (list)) (make)`
	exp = `(list) (list)`
	checkContainers(code, exp,
		&Container{ID: 1, Image: "a", Env: env},
		&Container{ID: 2, Image: "b", Env: env})

	// Test creating containers from within a module
	code = `(module "foo"
			  (define (Bar)
			    (docker "baz")))
			(foo.Bar)`
	exp = `(module "foo"
			 (list))
		   (docker "baz")`
	checkContainers(code, exp, &Container{ID: 1, Image: "baz", Env: env})

	runtimeErr(t, `(docker bar)`, `1: unassigned variable: bar`)
	runtimeErr(t, `(docker 1)`, `1: expected string, found: 1`)
}

func TestStdlib(t *testing.T) {
	// We need to specify that we read from disk because other tests may have switched
	// the filesystem to MemMap.
	util.AppFs = afero.NewOsFs()

	checkMachines := func(code, expectedCode string, expected ...Machine) {
		ctx := parseTestImport(t, code, expectedCode, []string{"../specs/stdlib"})
		machineResult := Stitch{"", ctx}.QueryMachines()
		if !reflect.DeepEqual(machineResult, expected) {
			t.Error(spew.Sprintf("test: %s, result: %v, expected: %v",
				code, machineResult, expected))
		}
	}

	code := `(import "machines")
	         (machines.Boot
	           1
	           2
			   (list (provider "Amazon")
					 (size "m4.large")
					 (sshkey "key")))`
	expCode := `(module "machines" (list)
                (list)
                (list)
                (list))
                (list (machine (provider "Amazon") (size "m4.large") (role "Master")
                        (sshkey "key"))
                      (machine (provider "Amazon") (size "m4.large") (role "Worker")
                        (sshkey "key"))
                      (machine (provider "Amazon") (size "m4.large") (role "Worker")
                        (sshkey "key")))`
	expMachines := []Machine{
		{Provider: "Amazon",
			Size:    "m4.large",
			Role:    "Master",
			SSHKeys: []string{"key"},
		},
		{Provider: "Amazon",
			Size:    "m4.large",
			Role:    "Worker",
			SSHKeys: []string{"key"},
		},
		{Provider: "Amazon",
			Size:    "m4.large",
			Role:    "Worker",
			SSHKeys: []string{"key"},
		},
	}
	checkMachines(code, expCode, expMachines...)
}

func TestMachines(t *testing.T) {
	checkMachines := func(code, expectedCode string, expected ...Machine) {
		ctx := parseTest(t, code, expectedCode)
		machineResult := Stitch{"", ctx}.QueryMachines()
		if !reflect.DeepEqual(machineResult, expected) {
			t.Error(spew.Sprintf("test: %s, result: %v, expected: %v",
				code, machineResult, expected))
		}
	}

	// Test no attributes
	code := `(machine)`
	expMachine := Machine{}
	checkMachines(code, code, expMachine)

	// Test specifying the provider
	code = `(machine (provider "Amazon"))`
	expMachine = Machine{Provider: "Amazon"}
	checkMachines(code, code, expMachine)

	// Test making a list of machines
	code = `(makeList 2 (machine (provider "Amazon")))`
	expCode := `(list (machine (provider "Amazon")) (machine (provider "Amazon")))`
	checkMachines(code, expCode, expMachine, expMachine)

	expMachine = Machine{Provider: "Amazon", Size: "m4.large"}
	code = `(machine (provider "Amazon") (size "m4.large"))`
	checkMachines(code, code, expMachine)

	// Test heterogeneous sizes
	code = `(machine (provider "Amazon") (size "m4.large"))
	        (machine (provider "Amazon") (size "m4.xlarge"))`
	expMachine2 := Machine{Provider: "Amazon", Size: "m4.xlarge"}
	checkMachines(code, code, expMachine, expMachine2)

	// Test heterogeneous providers
	code = `(machine (provider "Amazon") (size "m4.large"))
	        (machine (provider "Vagrant"))`
	expMachine2 = Machine{Provider: "Vagrant"}
	checkMachines(code, code, expMachine, expMachine2)

	// Test cpu range (two args)
	code = `(machine (provider "Amazon") (cpu 4 8))`
	expMachine = Machine{Provider: "Amazon", CPU: Range{Min: 4, Max: 8}}
	checkMachines(code, code, expMachine)

	// Test cpu range (one arg)
	code = `(machine (provider "Amazon") (cpu 4))`
	expMachine = Machine{Provider: "Amazon", CPU: Range{Min: 4}}
	checkMachines(code, code, expMachine)

	// Test ram range
	code = `(machine (provider "Amazon") (ram 8 12))`
	expMachine = Machine{Provider: "Amazon", RAM: Range{Min: 8, Max: 12}}
	checkMachines(code, code, expMachine)

	// Test float range
	code = `(machine (provider "Amazon") (ram 0.5 2))`
	expMachine = Machine{Provider: "Amazon", RAM: Range{Min: 0.5, Max: 2}}
	checkMachines(code, code, expMachine)

	// Test named attribute
	code = `(define large (list (ram 16) (cpu 8)))
	(machine (provider "Amazon") large)`
	expCode = `(list)
	(machine (provider "Amazon") (ram 16) (cpu 8))`
	expMachine = Machine{Provider: "Amazon", RAM: Range{Min: 16}, CPU: Range{Min: 8}}
	checkMachines(code, expCode, expMachine)

	// Test setting region
	code = `(machine (region "us-west-1"))`
	expMachine = Machine{Region: "us-west-1"}
	checkMachines(code, code, expMachine)

	// Test setting disk size
	code = `(machine (diskSize 32))`
	expMachine = Machine{DiskSize: 32}
	checkMachines(code, code, expMachine)

	// Test invalid attribute type
	runtimeErr(t, `(machine (provider "Amazon") "foo")`,
		`1: unrecognized argument to machine definition: "foo"`)
}

func TestMachineAttribute(t *testing.T) {
	checkMachines := func(code, expectedCode string, expected ...Machine) {
		ctx := parseTest(t, code, expectedCode)
		machineResult := Stitch{"", ctx}.QueryMachines()
		if !reflect.DeepEqual(machineResult, expected) {
			t.Error(spew.Sprintf("test: %s, result: %v, expected: %v",
				code, machineResult, expected))
		}
	}

	// Test adding an attribute to an empty machine definition
	code := `(define machines (list (machine)))
	(machineAttribute machines (provider "Amazon"))`
	expCode := `(list)
	(list (machine (provider "Amazon")))`
	expMachine := Machine{Provider: "Amazon"}
	checkMachines(code, expCode, expMachine)

	// Test adding an attribute to a machine that already has another attribute
	code = `(define machines (list (machine (size "m4.large"))))
	(machineAttribute machines (provider "Amazon"))`
	expCode = `(list)
	(list (machine (provider "Amazon") (size "m4.large")))`
	expMachine = Machine{Provider: "Amazon", Size: "m4.large"}
	checkMachines(code, expCode, expMachine)

	// Test adding two attributes
	code = `(define machines (list (machine)))
	(machineAttribute machines (provider "Amazon") (size "m4.large"))`
	expCode = `(list)
	(list (machine (provider "Amazon") (size "m4.large")))`
	expMachine = Machine{Provider: "Amazon", Size: "m4.large"}
	checkMachines(code, expCode, expMachine)

	// Test replacing an attribute
	code = `(define machines (list (machine (provider "Amazon") (size "m4.large"))))
	(machineAttribute machines (size "m4.medium"))`
	expCode = `(list)
	(list (machine (provider "Amazon") (size "m4.medium")))`
	expMachine = Machine{Provider: "Amazon", Size: "m4.medium"}
	checkMachines(code, expCode, expMachine)

	// Test setting attributes on a single machine (non-list)
	code = `(define machines (machine (provider "Amazon")))
	(machineAttribute machines (size "m4.medium"))`
	expCode = `(list)
	(list (machine (provider "Amazon") (size "m4.medium")))`
	expMachine = Machine{Provider: "Amazon", Size: "m4.medium"}
	checkMachines(code, expCode, expMachine)

	// Test setting attribute on a non-machine
	code = `(define badMachine (docker "foo"))
	(machineAttribute badMachine (machine (provider "Amazon")))`
	runtimeErr(t, code,
		fmt.Sprintf(`2: bad type, cannot change machine attributes: (docker "foo")`))

	// Test setting range attributes
	code = `(define machines (machine (provider "Amazon")))
	(machineAttribute machines (ram 1))`
	expCode = `(list)
	(list (machine (provider "Amazon") (ram 1)))`
	expMachine = Machine{Provider: "Amazon", RAM: Range{Min: 1}}
	checkMachines(code, expCode, expMachine)

	// Test setting using a named attribute
	code = `(define large (list (ram 16) (cpu 8)))
	(define machines (machine (provider "Amazon")))
	(machineAttribute machines large)`
	expCode = `(list)
	(list)
	(list (machine (provider "Amazon") (ram 16) (cpu 8)))`
	expMachine = Machine{Provider: "Amazon", RAM: Range{Min: 16}, CPU: Range{Min: 8}}
	checkMachines(code, expCode, expMachine)

	// Test setting attributes from within a lambda
	code = `(define machines (machine (provider "Amazon")))
	(define makeLarge (lambda (machines) (machineAttribute machines (ram 16) (cpu 8))))
	(makeLarge machines)`
	expCode = `(list)
	(list)
	(list (machine (provider "Amazon") (ram 16) (cpu 8)))`
	expMachine = Machine{Provider: "Amazon", RAM: Range{Min: 16}, CPU: Range{Min: 8}}
	checkMachines(code, expCode, expMachine)
}

func TestKeys(t *testing.T) {
	getGithubKeys = func(username string) ([]string, error) {
		return []string{username}, nil
	}

	checkKeys := func(code, expectedCode string, expected ...string) {
		ctx := parseTest(t, code, expectedCode)
		machineResult := Stitch{"", ctx}.QueryMachines()
		if len(machineResult) == 0 {
			t.Error("no machine found")
			return
		}
		if !reflect.DeepEqual(machineResult[0].SSHKeys, expected) {
			t.Error(spew.Sprintf("test: %s, result: %s, expected: %s",
				code, machineResult[0].SSHKeys, expected))
		}
	}

	code := `(machine (sshkey "key"))`
	checkKeys(code, code, "key")

	code = `(machine (githubKey "user"))`
	checkKeys(code, code, "user")

	code = `(machine (githubKey "user") (sshkey "key"))`
	checkKeys(code, code, "user", "key")
}

func TestLabel(t *testing.T) {
	env := map[string]string{}

	code := `(label "foo" (docker "a"))
	(label "bar" "foo" (docker "b"))
	(label "baz" "foo" "bar")
	(label "baz2" "baz")
	(label "qux" (docker "c"))`
	expCode := `(label "foo" (docker "a"))
	(label "bar" (docker "a") (docker "b"))
	(label "baz" (docker "a") (docker "b"))
	(label "baz2" (docker "a") (docker "b"))
	(label "qux" (docker "c"))`
	ctx := parseTest(t, code, expCode)
	stitch := Stitch{"", ctx}

	containerA := &Container{ID: 1, Image: "a", Command: nil, Env: env}
	containerB := &Container{ID: 2, Image: "b", Command: nil, Env: env}
	containerC := &Container{ID: 3, Image: "c", Command: nil, Env: env}
	expected := []*Container{containerA, containerB, containerC}
	containerResult := stitch.QueryContainers()
	if !reflect.DeepEqual(containerResult, expected) {
		t.Error(spew.Sprintf("\ntest: %v\nresult  : %v\nexpected: %v",
			code, containerResult, expected))
	}

	expLabels := map[string][]int{
		"foo":  {1},
		"bar":  {1, 2},
		"baz":  {1, 2},
		"baz2": {1, 2},
		"qux":  {3},
	}
	labels := stitch.QueryLabels()
	if !reflect.DeepEqual(labels, expLabels) {
		t.Error(spew.Sprintf("\ntest labels: %v\nresult  : %v\nexpected: %v",
			code, labels, expLabels))
	}

	code = `(label "foo" (makeList 2 (docker "a")))` +
		"\n(label \"bar\" \"foo\")"
	exp := `(label "foo" (docker "a") (docker "a"))
	(label "bar" (docker "a") (docker "a"))`
	ctx = parseTest(t, code, exp)
	stitch = Stitch{"", ctx}

	a1 := Container{ID: 1, Image: "a", Command: nil, Env: env}
	a2 := a1
	a2.ID = 2
	expected = []*Container{&a1, &a2}
	containerResult = stitch.QueryContainers()
	if !reflect.DeepEqual(containerResult, expected) {
		t.Error(spew.Sprintf("\ntest: %v\nresult  : %v\nexpected: %v",
			code, containerResult, expected))
	}

	expLabels = map[string][]int{
		"foo": {1, 2},
		"bar": {1, 2},
	}
	labels = stitch.QueryLabels()
	if !reflect.DeepEqual(labels, expLabels) {
		t.Error(spew.Sprintf("\ntest labels: %v\nresult  : %v\nexpected: %v",
			code, labels, expLabels))
	}

	// Test referring to a label directly
	code = `(define myMachines (machine))
	(machineAttribute myMachines (provider "Amazon"))`
	exp = `(list)
	(list (machine (provider "Amazon")))`
	ctx = parseTest(t, code, exp)
	expMachines := []Machine{
		{
			Provider: "Amazon",
		},
	}
	machineResult := Stitch{"", ctx}.QueryMachines()
	if !reflect.DeepEqual(machineResult, expMachines) {
		t.Error(spew.Sprintf("\ntest: %s\nresult  : %v\nexpected: %v",
			code, machineResult, expMachines))
	}

	// Test getting label name
	code = `(define foo (label "bar" (docker "baz"))) (labelName foo)`
	exp = `(list) "bar"`
	parseTest(t, code, exp)

	// Test getting label Hostname
	code = `(define foo (label "bar" (docker "baz"))) (labelHost foo)`
	exp = `(list) "bar.q"`
	parseTest(t, code, exp)

	runtimeErr(t, `(label 1 2)`, "1: label must be a string, found: 1")
	runtimeErr(t, `(label "foo" "bar")`, `1: undefined label: "bar"`)
	runtimeErr(t, `(label "foo" 1)`,
		"1: label must apply to atoms or other labels, found: 1")
	runtimeErr(t, `(label "foo" (docker "a")) (label "foo" "foo")`,
		"1: attempt to redefine label: foo")
}

func TestPlacement(t *testing.T) {
	checkPlacement := func(code, expCode string, expected ...Placement) {
		ctx := parseTest(t, code, expCode)
		placementResult := Stitch{"", ctx}.QueryPlacements()
		if !reflect.DeepEqual(placementResult, expected) {
			t.Error(spew.Sprintf("test: %s, result: %v, expected: %v",
				code, placementResult, expected))
		}
	}

	// Test exclusive labels
	code := `(label "red" (docker "foo"))
	(label "blue" (docker "bar"))
	(place (labelRule "exclusive" "red") "blue")`
	expCode := `(label "red" (docker "foo"))
	(label "blue" (docker "bar"))
	(list)`
	checkPlacement(code, expCode,
		Placement{
			TargetLabel: "blue",
			Exclusive:   true,
			OtherLabel:  "red",
		},
	)

	// Test paired labels
	code = `(label "red" (docker "foo"))
	(label "blue" (docker "bar"))
	(place (labelRule "on" "red") "blue")`
	expCode = `(label "red" (docker "foo"))
	(label "blue" (docker "bar"))
	(list)`
	checkPlacement(code, expCode,
		Placement{
			TargetLabel: "blue",
			Exclusive:   false,
			OtherLabel:  "red",
		})

	// Test placement by direct reference to labels
	code = `(place
	(labelRule "exclusive"
	  (label "red" (docker "foo")))
	(label "blue" (docker "bar")))`
	checkPlacement(code, "(list)",
		Placement{
			TargetLabel: "blue",
			Exclusive:   true,
			OtherLabel:  "red",
		},
	)

	// Test multiple target labels
	code = `(place (labelRule "exclusive" (label "red" (docker "foo")))
	               (label "blue" (docker "bar"))
	               (label "purple" (docker "baz")))`
	checkPlacement(code, "(list)",
		Placement{
			Exclusive:   true,
			TargetLabel: "blue",
			OtherLabel:  "red",
		},
		Placement{
			Exclusive:   true,
			TargetLabel: "purple",
			OtherLabel:  "red",
		})

	// Test machine rule
	code = `(place
	(machineRule "on" (region "us-west-2") (provider "AmazonSpot") (size "m4.large"))
	(label "blue" (docker "bar")))`
	checkPlacement(code, "(list)",
		Placement{
			TargetLabel: "blue",
			Exclusive:   false,
			Region:      "us-west-2",
			Provider:    "AmazonSpot",
			Size:        "m4.large",
		},
	)
}

func TestEnv(t *testing.T) {
	code := `(label "red" (docker "a"))
	(setEnv "red" "key" "value")`
	expCode := `(label "red" (docker "a"))
	(list)`
	ctx := parseTest(t, code, expCode)
	containerA := Container{
		ID:    1,
		Image: "a",
		Env:   map[string]string{"key": "value"}}
	expected := []*Container{&containerA}
	containerResult := Stitch{"", ctx}.QueryContainers()
	if !reflect.DeepEqual(containerResult, expected) {
		t.Error(spew.Sprintf("\ntest: %v\nresult  : %v\nexpected: %v",
			code, containerResult, expected))
	}

	code = `(label "red" (makeList 5 (docker "a")))
	(setEnv "red" "key" "value")`
	expCode = `(label "red"
	  (docker "a") (docker "a") (docker "a") (docker "a") (docker "a"))
	(list)`
	ctx = parseTest(t, code, expCode)

	expected = nil
	for i := 1; i <= 5; i++ {
		expected = append(expected, &Container{
			ID:    i,
			Image: "a",
			Env:   map[string]string{"key": "value"}})
	}
	containerResult = Stitch{"", ctx}.QueryContainers()
	if !reflect.DeepEqual(containerResult, expected) {
		t.Error(spew.Sprintf("\ntest: %v\nresult  : %v\nexpected: %v",
			code, containerResult, expected))
	}

	code = `(label "foo" (docker "a"))
	(label "bar" "foo" (docker "b"))
	(label "baz" "foo" "bar")
	(setEnv "bar" "key1" "value1")
	(setEnv "baz" "key2" "value2")`
	expCode = `(label "foo" (docker "a"))
	(label "bar" (docker "a") (docker "b"))
	(label "baz" (docker "a") (docker "b"))
	(list)
	(list)`
	ctx = parseTest(t, code, expCode)
	containerA = Container{
		ID:    1,
		Image: "a",
		Env:   map[string]string{"key1": "value1", "key2": "value2"}}
	containerB := Container{
		ID:    2,
		Image: "b",
		Env:   map[string]string{"key1": "value1", "key2": "value2"}}
	expected = []*Container{&containerA, &containerB}
	containerResult = Stitch{"", ctx}.QueryContainers()
	if !reflect.DeepEqual(containerResult, expected) {
		t.Error(spew.Sprintf("\ntest: %v\nresult  : %v\nexpected: %v",
			code, containerResult, expected))
	}

	code = `(setEnv (list (docker "a")) "key" "value")`
	expCode = `(list)`
	ctx = parseTest(t, code, expCode)
	containerA = Container{
		ID:    1,
		Image: "a",
		Env:   map[string]string{"key": "value"}}
	expected = []*Container{&containerA}
	containerResult = Stitch{"", ctx}.QueryContainers()
	if !reflect.DeepEqual(containerResult, expected) {
		t.Error(spew.Sprintf("\ntest: %v\nresult  : %v\nexpected: %v",
			code, containerResult, expected))
	}

	code = `(let ((foo (label "bar" (docker "a"))))
	(setEnv foo "key" "value"))`
	ctx = parseTest(t, code, "(list)")
	containerA = Container{
		ID:    1,
		Image: "a",
		Env:   map[string]string{"key": "value"},
	}
	expected = []*Container{&containerA}
	containerResult = Stitch{"", ctx}.QueryContainers()
	if !reflect.DeepEqual(containerResult, expected) {
		t.Error(spew.Sprintf("\ntest: %v\nresult  : %v\nexpected: %v",
			code, containerResult, expected))
	}

	runtimeErr(t, `(setEnv (docker "foo") 1 "value")`, "1: setEnv key must be a string: 1")
	runtimeErr(t, `(setEnv (docker "foo") "key" 1)`, "1: setEnv value must be a string: 1")
}

func TestConnect(t *testing.T) {
	code := `(progn
	(label "a" (docker "alpine"))
	(label "b" (docker "alpine"))
	(label "c" (docker "alpine"))
	(label "d" (docker "alpine"))
	(label "e" (docker "alpine"))
	(label "f" (docker "alpine"))
	(label "g" (docker "alpine"))
	(label "h" (docker "alpine"))
	(connect 80 "a" "b")
	(connect 80 "a" "b" "c")
	(connect (list 1 65534) "b" "c")
	(connect (list 0 65535) "a" "c")
	(connect 443 "c" "d" "e" "f")
	((lambda () (connect 80 "h" "h")))
	(let ((i (label "i" (docker "alpine"))))
	  (connect 80  i i))
	(connect (list 100 65535) "g" "g"))`
	ctx := parseTest(t, code, `(list)`)

	expected := map[Connection]struct{}{
		{"a", "b", 80, 80}:     {},
		{"a", "c", 80, 80}:     {},
		{"b", "c", 1, 65534}:   {},
		{"a", "c", 0, 65535}:   {},
		{"c", "d", 443, 443}:   {},
		{"c", "e", 443, 443}:   {},
		{"c", "f", 443, 443}:   {},
		{"h", "h", 80, 80}:     {},
		{"g", "g", 100, 65535}: {},
		{"i", "i", 80, 80}:     {},
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

	runtimeErr(t, `(connect a "foo" "bar")`, "1: unassigned variable: a")
	runtimeErr(t, `(connect (list 80) "foo" "bar")`,
		"1: port range must have two ints: (list 80)")
	runtimeErr(t, `(connect (list 0 70000) "foo" "bar")`,
		"1: invalid port range: [0, 70000]")
	runtimeErr(t, `(connect (list (- 0 10) 10) "foo" "bar")`,
		"1: invalid port range: [-10, 10]")
	runtimeErr(t, `(connect (list 100 10) "foo" "bar")`,
		"1: invalid port range: [100, 10]")
	runtimeErr(t, `(connect "80" "foo" "bar")`,
		"1: port range must be an int or a list of ints: \"80\"")
	runtimeErr(t, `(connect (list "a" "b") "foo" "bar")`,
		"1: port range must have two ints: (list \"a\" \"b\")")
	runtimeErr(t, `(connect 80 4 5)`, "1: expected label, found: 4")
	runtimeErr(t, `(connect 80 "foo" "foo")`, "1: expected label, found: \"foo\"")
}

func TestPublic(t *testing.T) {
	code := `(progn
                 (label "foo" (docker "foo"))
                 (label "bar" (docker "bar"))
                 (label "baz" (docker "baz"))
                 (connect 80 "public" "foo")
                 (connect 80 "bar" "public")
                 ((lambda ()
                          (connect 81 "bar" "public")
                          (connect 81 "public" "baz"))))`
	ctx := parseTest(t, code, `(list)`)

	expCon := map[Connection]struct{}{
		Connection{
			From:    "public",
			To:      "foo",
			MinPort: 80,
			MaxPort: 80,
		}: {},

		Connection{
			From:    "bar",
			To:      "public",
			MinPort: 80,
			MaxPort: 80,
		}: {},

		Connection{
			From:    "bar",
			To:      "public",
			MinPort: 81,
			MaxPort: 81,
		}: {},

		Connection{
			From:    "public",
			To:      "baz",
			MinPort: 81,
			MaxPort: 81,
		}: {},
	}
	if !reflect.DeepEqual(ctx.connections, expCon) {
		t.Error(spew.Sprintf("Public connection \nexp %v,\ngot %v",
			expCon, ctx.connections))
	}

	expPlace := map[Placement]struct{}{
		Placement{
			Exclusive:   true,
			TargetLabel: "foo",
			OtherLabel:  "foo",
		}: {},

		Placement{
			Exclusive:   true,
			TargetLabel: "bar",
			OtherLabel:  "foo",
		}: {},

		Placement{
			Exclusive:   true,
			TargetLabel: "bar",
			OtherLabel:  "bar",
		}: {},

		Placement{
			Exclusive:   true,
			TargetLabel: "baz",
			OtherLabel:  "baz",
		}: {},

		Placement{
			Exclusive:   true,
			TargetLabel: "baz",
			OtherLabel:  "bar",
		}: {},
	}
	if !reflect.DeepEqual(ctx.placements, expPlace) {
		t.Error(spew.Sprintf("Public placement \nexp %v,\ngot %v",
			expPlace, ctx.placements))
	}

	runtimeErr(t, `(label "bar" (docker "bar"))
		(connect (list 80 90) "public" "bar")`,
		"2: public internet cannot connect on port ranges")

	runtimeErr(t, `(connect 80 "public" "public")`,
		"1: cannot connect public internet to itself")
}

func TestImport(t *testing.T) {
	// Test module keyword
	code := `(module "math" (define Square (lambda (x) (* x x)))) (math.Square 2)`
	parseTest(t, code, `(module "math" (list)) 4`)

	// Test module with multiple-statement body
	parseTest(t,
		`(module "math" (define three 3)
		(define Triple (lambda (x) (* three x))))
		(math.Triple 2)`, `(module "math" (list) (list)) 6`)

	// Test importing from disk
	testFs := afero.NewMemMapFs()
	util.AppFs = testFs
	util.WriteFile("math.spec", []byte("(define Square (lambda (x) (* x x)))"), 0644)
	util.AppFs = testFs
	parseTestImport(t, `(import "math") (math.Square 2)`, `(module "math" (list)) 4`,
		[]string{"."})

	// Test two imports in separate directories
	testFs = afero.NewMemMapFs()
	util.AppFs = testFs
	testFs.Mkdir("square", 777)
	util.WriteFile("square/square.spec",
		[]byte("(define Square (lambda (x) (* x x)))"), 0644)
	testFs.Mkdir("cube", 777)
	util.WriteFile("cube/cube.spec",
		[]byte("(define Cube (lambda (x) (* x x x)))"), 0644)
	parseTestImport(t,
		`(import "square")
		(import "cube")
		(square.Square 2)
		(cube.Cube 2)`,
		`(module "square" (list))
		(module "cube" (list)) 4 8`,
		[]string{"square", "cube"})

	// Test import with an import
	testFs = afero.NewMemMapFs()
	util.AppFs = testFs
	util.WriteFile("square.spec", []byte("(define Square (lambda (x) (* x x)))"), 0644)
	util.WriteFile("cube.spec", []byte(`(import "square")
	(define Cube (lambda (x) (* x (square.Square x))))`), 0644)
	parseTestImport(t, `(import "cube") (cube.Cube 2)`,
		`(module "cube" (module "square" (list)) (list)) 8`,
		[]string{"."})

	// Test error in an imported module
	testFs = afero.NewMemMapFs()
	util.AppFs = testFs
	util.WriteFile("bad.spec", []byte(`(define BadFunc (lambda () (+ 1 "1")))`), 0644)
	runtimeErrImport(t, `(import "bad") (bad.BadFunc)`,
		`./bad.spec:1: bad arithmetic argument: "1"`, []string{"."})

	testFs = afero.NewMemMapFs()
	util.AppFs = testFs
	util.WriteFile("A.spec", []byte(`(import "A")`), 0644)
	importErr(t, `(import "A")`, `import cycle: [A A]`, []string{"."})

	testFs = afero.NewMemMapFs()
	util.AppFs = testFs
	util.WriteFile("A.spec", []byte(`(import "B")`), 0644)
	util.WriteFile("B.spec", []byte(`(import "A")`), 0644)
	importErr(t, `(import "A")`, `import cycle: [A B A]`, []string{"."})

	testFs = afero.NewMemMapFs()
	util.AppFs = testFs
	util.WriteFile("A.spec", []byte(`(import "B")
	(import "C")
	(define AddTwo (lambda (x) (+ (B.AddOne x) C.One)))`), 0644)
	util.WriteFile("B.spec", []byte(`(import "C")
	(define AddOne (lambda (x) (+ x C.One)))`), 0644)
	util.WriteFile("C.spec", []byte(`(define One 1)`), 0644)
	parseTestImport(t, `(import "A") (A.AddTwo 1)`,
		`(module "A" (module "B" (module "C" (list)) (list))
		(module "C" (list))
		(list)) 3`, []string{"."})

	// Test that non-capitalized binds are not exported
	runtimeErr(t, `(module "A" (define addOne (lambda (x) (+ x 1))))
(A.addOne 1)`, `2: unknown function: A.addOne`)
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
	err := `1: bad arithmetic argument: (list)`
	runtimeErr(t, `(+ (list) (list))`, err)
	runtimeErr(t, `(list (+ (list) (list)))`, err)
	runtimeErr(t, `(let ((y (+ (list) (list)))) y)`, err)
	runtimeErr(t, `(let ((y 3)) (+ (list) (list)))`, err)

	runtimeErr(t, "(define a 3) (define a 3)", `1: attempt to redefine: "a"`)
	runtimeErr(t, "(define a (+ 3 b ))", "1: unassigned variable: b")

	runtimeErr(t, `(makeList a 3)`, "1: unassigned variable: a")
	runtimeErr(t, `(makeList 3 a)`, "1: unassigned variable: a")
	runtimeErr(t, `(makeList "a" 3)`,
		`1: makeList must begin with a positive integer, found: "a"`)

	runtimeErr(t, `(label a a)`, "1: unassigned variable: a")

	runtimeErr(t, "(1 2 3)", "1: s-expressions must start with a function call: 1")

	args := "1: not enough arguments: +"
	runtimeErr(t, "(+)", args)
	runtimeErr(t, "(+ 5)", args)
	runtimeErr(t, "(+ 5 (+ 6))", args)

	runtimeErr(t, "()", "1: s-expressions must start with a function call: ()")
	runtimeErr(t, "(let)", "1: not enough arguments: let")
	runtimeErr(t, "(let 3 a)", "1: let binds must be defined in an S-expression")
	runtimeErr(t, "(let (a) a)", "1: binds must be exactly 2 arguments: a")
	runtimeErr(t, "(let ((a)) a)", "1: binds must be exactly 2 arguments: (a)")
	runtimeErr(t, "(let ((3 a)) a)", "1: bind name must be an ident: 3")
	runtimeErr(t, "(let ((a (+))) a)", args)
	runtimeErr(t, "(let ((a 3)) (+))", args)

	runtimeErr(t, "(define a (+))", args)

	runtimeErr(t, "(badFun)", "1: unknown function: badFun")

	runtimeErr(t, `(panic "foo")`, "1: panic: runtime error: foo")
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

		_, _, err = eval(astRoot(parsed))
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

	// Test that functions which evaluate generated S-expressions still have proper
	// error messages.
	util.WriteFile("generated_sexp.spec", []byte(`(apply + (list 1 "1"))`), 0644)
	checkEvalErr("generated_sexp.spec",
		`generated_sexp.spec:1: bad arithmetic argument: "1"`)
}

func TestQuery(t *testing.T) {
	var sc scanner.Scanner
	stitch, err := New(*sc.Init(strings.NewReader("(")), []string{})
	if err == nil {
		t.Error("Expected error")
	}

	stitch, err = New(*sc.Init(strings.NewReader("(+ a a)")), []string{})
	if err == nil {
		t.Error("Expected runtime error")
	}

	stitch, err = New(*sc.Init(strings.NewReader(`
		(define a (+ 1 2))
		(define b "This is b")
		(define c (list "This" "is" "b"))
		(define d (list "1" 2 "3"))
		(define e 1.5)
		(docker b)
		(docker b)`)), []string{})
	if err != nil {
		t.Error(err)
		return
	}

	if val, _ := stitch.QueryFloat("e"); val != 1.5 {
		t.Error(val, "!=", 1.5)
	}

	if val := stitch.QueryString("b"); val != "This is b" {
		t.Error(val, "!=", "This is b")
	}

	if val := stitch.QueryString("missing"); val != "" {
		t.Error(val, "!=", "")
	}

	if val := stitch.QueryString("a"); val != "" {
		t.Error(val, "!=", "")
	}

	expected := []string{"This", "is", "b"}
	if val := stitch.QueryStrSlice("c"); !reflect.DeepEqual(val, expected) {
		t.Error(val, "!=", expected)
	}

	if val := stitch.QueryStrSlice("d"); val != nil {
		t.Error(val, "!=", nil)
	}

	if val := stitch.QueryStrSlice("missing"); val != nil {
		t.Error(val, "!=", nil)
	}

	if val := stitch.QueryStrSlice("a"); val != nil {
		t.Error(val, "!=", nil)
	}

	if val := stitch.QueryContainers(); len(val) != 2 {
		t.Error(val)
	}
}

func parseTestImport(t *testing.T, code, evalExpected string, path []string) evalCtx {
	var sc scanner.Scanner
	parsed, err := parse(*sc.Init(strings.NewReader(code)))
	if err != nil {
		t.Errorf("%s: %s", code, err)
		return evalCtx{}
	}

	parsed, err = resolveImports(parsed, path)
	if err != nil {
		t.Errorf("%s: %s", code, err)
		return evalCtx{}
	}

	return parseTestCheck(t, astRoot(parsed), astRoot(parsed).String(), evalExpected)
}

func parseTest(t *testing.T, code, evalExpected string) evalCtx {
	var sc scanner.Scanner
	parsed, err := parse(*sc.Init(strings.NewReader(code)))
	if err != nil {
		t.Errorf("%s: %s", code, err)
		return evalCtx{}
	}

	return parseTestCheck(t, astRoot(parsed), code, evalExpected)
}

func parseTestCheck(t *testing.T, parsed ast, code, evalExpected string) evalCtx {
	if str := parsed.String(); !codeEq(str, code) {
		t.Errorf("\nParse expected \"%s\"\ngot \"%s\"", code, str)
		return evalCtx{}
	}

	result, ctx, err := eval(parsed)
	if err != nil {
		t.Errorf("%s: %s", code, err)
		return evalCtx{}
	}

	if !codeEq(result.String(), evalExpected) {
		t.Errorf("\nEval expected \"%s\"\ngot \"%s\"", evalExpected, result)
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

func runtimeErr(t *testing.T, code, expectedErr string) {
	var sc scanner.Scanner
	prog, err := parse(*sc.Init(strings.NewReader(code)))
	if err != nil {
		t.Errorf("%s: %s", code, err)
		return
	}

	_, _, err = eval(astRoot(prog))
	if fmt.Sprintf("%s", err) != expectedErr {
		t.Errorf("%s: %s", code, err)
		return
	}
}

func runtimeErrImport(t *testing.T, code, expectedErr string, path []string) {
	var sc scanner.Scanner
	prog, err := parse(*sc.Init(strings.NewReader(code)))
	if err != nil {
		t.Errorf("%s: %s", code, err)
		return
	}

	prog, err = resolveImports(prog, path)
	if err != nil {
		t.Errorf("%s: %s", code, err)
		return
	}

	_, _, err = eval(astRoot(prog))
	if fmt.Sprintf("%s", err) != expectedErr {
		t.Errorf("%s: %s", code, err)
		return
	}
}

func importErr(t *testing.T, code, expectedErr string, path []string) {
	var sc scanner.Scanner
	prog, err := parse(*sc.Init(strings.NewReader(code)))
	if err != nil {
		t.Errorf("%s: %s", code, err)
		return
	}

	_, err = resolveImports(prog, path)
	if fmt.Sprintf("%s", err) != expectedErr {
		t.Errorf("%s\n\t%s: %s", code, err, expectedErr)
		return
	}
}

var codeEqRE = regexp.MustCompile(`\s+`)

func codeEq(a, b string) bool {
	a = strings.TrimSpace(codeEqRE.ReplaceAllString(a, " "))
	b = strings.TrimSpace(codeEqRE.ReplaceAllString(b, " "))
	return a == b
}
