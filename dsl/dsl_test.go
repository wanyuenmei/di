package dsl

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/davecgh/go-spew/spew"
)

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
}

func TestLet(t *testing.T) {
	parseTest(t, "(let ((a 5)) a)", "5")
	parseTest(t, "(+ 12 (let ((a 5)) a))", "17")
	parseTest(t, "(let ((a 5)) (* a a))", "25")
	parseTest(t, "(let ((a 5) (b 2)) (* a b))", "10")
	parseTest(t, "(let ((a 2)) (* (let ((a 3)) (* a a)) (* a a)))", "36")
}

func TestDefine(t *testing.T) {
	code := "(define a 1)"
	parseTest(t, code, code)

	code = "(define a 1)\n(define b 2)"
	parseTest(t, code, code)

	code = "(define a 1)\n3\n(define b 2)"
	parseTest(t, code, code)

	parseTest(t, "(define a (+ 5 7))", "(define a 12)")

	parseTest(t, "(define a (+ 5 7))", "(define a 12)")

	parseTest(t, "(define a (+ 1 1))\n(define b (* a 2))",
		"(define a 2)\n(define b 4)")
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
}

func TestAtom(t *testing.T) {
	checkContainers := func(code, expectedCode string, expStr ...string) {
		var expected []*Container
		for _, e := range expStr {
			expected = append(expected, &Container{e, nil})
		}

		ctx := parseTest(t, code, expectedCode)
		if !reflect.DeepEqual(ctx.containers, expected) {
			t.Error(spew.Sprintf("test: %s, result: %s, expected: %s",
				code, ctx.containers, expected))
		}
	}

	code := `(atom docker "a")`
	checkContainers(code, code, "a")

	code = "(atom docker \"a\")\n(atom docker \"a\")"
	checkContainers(code, code, "a", "a")

	code = `(makeList 2 (list (atom docker "a") (atom docker "b")))`
	exp := `(list (list (atom docker "a") (atom docker "b"))` +
		` (list (atom docker "a") (atom docker "b")))`
	checkContainers(code, exp, "a", "b", "a", "b")

	code = `(let ((a "foo") (b "bar")) (list (atom docker a) (atom docker b)))`
	exp = `(list (atom docker "foo") (atom docker "bar"))`
	checkContainers(code, exp, "foo", "bar")

	runtimeErr(t, `(atom foo "bar")`, `unknown atom type: foo`)
	runtimeErr(t, `(atom docker bar)`, `unassigned variable: bar`)
	runtimeErr(t, `(atom docker 1)`, `atom argument must be a string, found: 1`)
}

func TestLabel(t *testing.T) {
	code := `(label "foo" (atom docker "a"))
(label "bar" "foo" (atom docker "b"))
(label "baz" "foo" "bar")
(label "baz2" "baz")
(label "qux" (atom docker "c"))`
	ctx := parseTest(t, code, code)

	expected := []*Container{
		{"a", []string{"foo", "bar", "baz", "baz2"}},
		{"b", []string{"bar", "baz", "baz2"}},
		{"c", []string{"qux"}}}
	if !reflect.DeepEqual(ctx.containers, expected) {
		t.Error(spew.Sprintf("\ntest: %s\nresult: %s\nexpected: %s",
			code, ctx.containers, expected))
	}

	code = `(label "foo" (makeList 2 (atom docker "a")))` +
		"\n(label \"bar\" \"foo\")"
	exp := `(label "foo" (list (atom docker "a") (atom docker "a")))` +
		"\n(label \"bar\" \"foo\")"
	ctx = parseTest(t, code, exp)
	expected = []*Container{
		{"a", []string{"foo", "bar"}},
		{"a", []string{"foo", "bar"}}}
	if !reflect.DeepEqual(ctx.containers, expected) {
		t.Error(spew.Sprintf("\ntest: %s\nresult: %s\nexpected: %s",
			code, ctx.containers, expected))
	}

	runtimeErr(t, `(label 1 2)`, "label must be a string, found: 1")
	runtimeErr(t, `(label "foo" "bar")`, `undefined label: "bar"`)
	runtimeErr(t, `(label "foo" 1)`,
		"label must apply to atoms or other labels, found: 1")
	runtimeErr(t, `(label "foo" (atom docker "a")) (label "foo" "foo")`,
		"attempt to redefine label: foo")
}

func TestConnect(t *testing.T) {
	code := `(label "a" (atom docker "alpine"))
(label "b" (atom docker "alpine"))
(label "c" (atom docker "alpine"))
(label "d" (atom docker "alpine"))
(label "e" (atom docker "alpine"))
(label "f" (atom docker "alpine"))
(label "g" (atom docker "alpine"))
(connect 80 "a" "b")
(connect 80 "a" "b" "c")
(connect (list 1 65534) "b" "c")
(connect (list 0 65535) "a" "c")
(connect 443 "c" "d" "e" "f")
(connect (list 100 65535) "g" "g")`
	ctx := parseTest(t, code, code)

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
			t.Error(spew.Sprintf("Missing connection: %s", exp))
			continue
		}

		delete(ctx.connections, exp)
	}

	if len(ctx.connections) > 0 {
		t.Error(spew.Sprintf("Unexpected connections: %s", ctx.connections))
	}

	runtimeErr(t, `(connect a "foo" "bar")`, "unassigned variable: a")
	runtimeErr(t, `(connect (list 80) "foo" "bar")`,
		"port range must have two ints: (list 80)")
	runtimeErr(t, `(connect (list 0 70000) "foo" "bar")`,
		"invalid port range: [0, 70000]")
	runtimeErr(t, `(connect (list (- 0 10) 10) "foo" "bar")`,
		"invalid port range: [-10, 10]")
	runtimeErr(t, `(connect (list 100 10) "foo" "bar")`,
		"invalid port range: [100, 10]")
	runtimeErr(t, `(connect "80" "foo" "bar")`,
		"port range must be an int or a list of ints: \"80\"")
	runtimeErr(t, `(connect (list "a" "b") "foo" "bar")`,
		"port range must have two ints: (list \"a\" \"b\")")
	runtimeErr(t, `(connect 80 4 5)`, "connect applies to labels: 4")
	runtimeErr(t, `(connect 80 "foo" "foo")`, "connect undefined label: \"foo\"")
}

func TestScanError(t *testing.T) {
	parseErr(t, "\"foo", "literal not terminated")
}

func TestParseErrors(t *testing.T) {
	parseErr(t, "(1 2 3)", "expressions must begin with keywords")

	unbalanced := "unbalanced Parenthesis"
	parseErr(t, "(", unbalanced)
	parseErr(t, ")", unbalanced)
	parseErr(t, "())", unbalanced)
	parseErr(t, "(((())", unbalanced)
	parseErr(t, "((+ 5 (* 3 7)))))", unbalanced)

	args := "not enough arguments: +"
	parseErr(t, "(+)", args)
	parseErr(t, "(+ 5)", args)
	parseErr(t, "(+ 5 (+ 6))", args)

	bindErr := "error parsing bindings"
	parseErr(t, "()", "bad element: []")
	parseErr(t, "4.3", "bad element: 4.3")
	parseErr(t, "(let)", "not enough arguments: [let]")
	parseErr(t, "(let 3 a)", bindErr)
	parseErr(t, "(let (a) a)", bindErr)
	parseErr(t, "(let ((a)) a)", bindErr)
	parseErr(t, "(let ((3 a)) a)", bindErr)
	parseErr(t, "(let ((a (+))) a)", args)
	parseErr(t, "(let ((a 3)) (+))", args)

	parseErr(t, "(define a (+))", args)
	parseErr(t, "(define a 5.3)", "bad element: 5.3")

	parseErr(t, "(badFun)", "unknown function: badFun")
	parseErr(t, "(atom)", "error parsing bindings")
}

func TestRuntimeErrors(t *testing.T) {
	err := `bad arithmetic argument: "a"`
	runtimeErr(t, `(+ "a" "a")`, err)
	runtimeErr(t, `(list (+ "a" "a"))`, err)
	runtimeErr(t, `(let ((y (+ "a" "a"))) y)`, err)
	runtimeErr(t, `(let ((y 3)) (+ "a" "a"))`, err)

	runtimeErr(t, "(define a 3) (define a 3)", `attempt to redefine: "a"`)
	runtimeErr(t, "(define a (+ 3 b ))", "unassigned variable: b")

	runtimeErr(t, `(makeList a 3)`, "unassigned variable: a")
	runtimeErr(t, `(makeList 3 a)`, "unassigned variable: a")
	runtimeErr(t, `(makeList "a" 3)`,
		`makeList must begin with a positive integer, found: "a"`)

	runtimeErr(t, `(label a a)`, "unassigned variable: a")
}

func TestQuery(t *testing.T) {
	dsl, err := New(strings.NewReader("("))
	if err == nil {
		t.Error("Expected error")
	}

	dsl, err = New(strings.NewReader("(+ a a)"))
	if err == nil {
		t.Error("Expected runtime error")
	}

	dsl, err = New(strings.NewReader(`
		(define a (+ 1 2))
		(define b "This is b")
		(define c (list "This" "is" "b"))
		(define d (list "1" 2 "3"))
		(atom docker b)
		(atom docker b)`))
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
}

func parseTest(t *testing.T, code, evalExpected string) evalCtx {
	parsed, err := parse(strings.NewReader(code))
	if err != nil {
		t.Error(fmt.Sprintf("%s: %s", code, err))
		return evalCtx{}
	}

	if str := parsed.String(); str != code {
		t.Errorf("\nParse expected \"%s\"\ngot \"%s\"", code, str)
		return evalCtx{}
	}

	result, ctx, err := eval(parsed)
	if err != nil {
		t.Error(fmt.Sprintf("%s: %s", code, err))
		return evalCtx{}
	}

	if result.String() != evalExpected {
		t.Errorf("\nEval expected \"%s\"\ngot \"%s\"", evalExpected, result)
		return evalCtx{}
	}

	// The code may be re-evaluated by the minions.  If that happens, the result
	// should be exactly the same.
	eval2, _, err := eval(result)
	if err != nil {
		t.Error(fmt.Sprintf("%s: %s", code, err))
		return evalCtx{}
	}

	if eval2.String() != result.String() {
		t.Errorf("\nEval expected \"%s\"\ngot \"%s\"", result, eval2)
		return evalCtx{}
	}

	return ctx
}

func parseErr(t *testing.T, code, expectedErr string) {
	_, err := parse(strings.NewReader(code))
	if fmt.Sprintf("%s", err) != expectedErr {
		t.Errorf("%s: %s", code, err)
	}
}

func runtimeErr(t *testing.T, code, expectedErr string) {
	prog, err := parse(strings.NewReader(code))
	if err != nil {
		t.Error(fmt.Sprintf("%s: %s", code, err))
		return
	}

	_, _, err = eval(prog)
	if fmt.Sprintf("%s", err) != expectedErr {
		t.Errorf("%s: %s", code, err)
		return
	}
}
