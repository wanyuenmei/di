package dsl

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
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
	parseTest(t, `"foo"`, `"foo"`)
	parseTest(t, `"foo" "bar"`, `"foo" "bar"`)
	parseTest(t, `"foo" 5 "bar"`, `"foo" 5 "bar"`)
}

func TestLet(t *testing.T) {
	parseTest(t, "(let ((a 5)) a)", "5")
	parseTest(t, "(+ 12 (let ((a 5)) a))", "17")
	parseTest(t, "(let ((a 5)) (* a a))", "25")
	parseTest(t, "(let ((a 5) (b 2)) (* a b))", "10")
	parseTest(t, "(let ((a 2)) (* (let ((a 3)) (* a a)) (* a a)))", "36")
}

func TestDefine(t *testing.T) {
	parseTest(t, "(define a 1)", "(define a 1)")
	parseTest(t, "(define a 1) (define b 2)", "(define a 1) (define b 2)")
	parseTest(t, "(define a 1) 3 (define b 2)", "(define a 1) 3 (define b 2)")
	parseTest(t, "(define a (+ 5 7))", "(define a 12)")
	parseTest(t, "(define a (+ 5 7))", "(define a 12)")
	parseTest(t, "(define a (+ 1 1)) (define b (* a 2))",
		"(define a 2) (define b 4)")
}

func TestList(t *testing.T) {
	parseTest(t, "(list)", "()")
	parseTest(t, "(list 1)", "(1)")
	parseTest(t, "(list 1 2)", "(1 2)")
	parseTest(t, "(list 1 2 (list))", "(1 2 ())")
	parseTest(t, `(list 1 2 (list "a" "b" 3))`, `(1 2 ("a" "b" 3))`)
	parseTest(t, `(list 1 2 (+ 1 2))`, `(1 2 3)`)

	parseTest(t, `(makeList 0 1)`, `()`)
	parseTest(t, `(makeList 1 1)`, `(1)`)
	parseTest(t, `(makeList 2 1)`, `(1 1)`)
	parseTest(t, `(makeList 3 (+ 1 1))`, `(2 2 2)`)
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

	parseErr(t, "(let ((a 1)) (define b a))", "define must be at the top level")
	parseErr(t, "(define a (+))", args)
	parseErr(t, "(define a 5.3)", "bad element: 5.3")

	parseErr(t, "(badFun)", "unknown function: badFun")
}

func TestRuntimeErrors(t *testing.T) {
	err := `bad arithmetic argument: "a"`
	runtimeErr(t, `(+ "a" "a")`, err)
	runtimeErr(t, `(list (+ "a" "a"))`, err)
	runtimeErr(t, `(let ((y (+ "a" "a"))) y)`, err)
	runtimeErr(t, `(let ((y 3)) (+ "a" "a"))`, err)

	runtimeErr(t, "(define a 3) (define a 3)", `attempt to redefine: "a"`)
	runtimeErr(t, "(define a (+ 3 b ))", "unassigned variable: b")

	runtimeErr(t, `(makeList "a" 3)`,
		`makeList must begin with a positive integer, found: "a"`)
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
		(define d (list "1" 2 "3"))`))
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
}

func parseTest(t *testing.T, code, evalExpected string) {
	parsed, err := parse(strings.NewReader(code))
	if err != nil {
		t.Error(fmt.Sprintf("%s: %s", code, err))
		return
	}

	if str := parsed.String(); str != code {
		t.Errorf("Parse expected \"%s\" got \"%s\"", code, str)
		return
	}

	result, err := parsed.eval(make(map[astIdent]ast))
	if err != nil {
		t.Error(fmt.Sprintf("%s: %s", code, err))
		return
	}

	if result.String() != evalExpected {
		t.Errorf("Eval expected \"%s\" got \"%s\"", evalExpected, result)
		return
	}
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

	_, err = prog.eval(make(map[astIdent]ast))
	if fmt.Sprintf("%s", err) != expectedErr {
		t.Errorf("%s: %s", code, err)
		return
	}
}
