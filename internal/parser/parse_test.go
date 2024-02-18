package parser_test

import (
	"testing"

	"github.com/XJIeI5/calculator/internal/parser"
)

func TestIgoneSpace(t *testing.T) {
	compare(t, "2 - 2+2", "2 2 - 2 + ", false)
}

func TestProcedureOfActions(t *testing.T) {
	compare(t, "2 + 2 * 2", "2 2 2 * + ", false)
	compare(t, "200 / 5 + 1", "200 5 / 1 + ", false)
}

func TestChangeOrderByParens(t *testing.T) {
	compare(t, "(1 + 2 * 3) / 4", "1 2 3 * + 4 / ", false)
	compare(t, "1 + 2 * 3 / 4", "1 2 3 * 4 / + ", false)
}

func TestMultipleParens(t *testing.T) {
	compare(t, "(1 / (2 * 3) / 4) + 5", "1 2 3 * / 4 / 5 + ", false)
	compare(t, "(1 - 2) * (3 + 4) / (5 + 6)", "1 2 - 3 4 + * 5 6 + / ", false)
}

func TestParenErrors(t *testing.T) {
	compare(t, "1 * 2 + 3)", "", true)
	compare(t, "(1 * 2 + 3", "", true)
}

func TestBinaryOperandErrors(t *testing.T) {
	compare(t, "1 * * 2", "", true)
	compare(t, "(1 - 2 +) + 3", "", true)
	compare(t, "2 + 2 + ", "", true)
}

func TestFloatNumber(t *testing.T) {
	compare(t, "2.5 + 5", "2.5 5 + ", true)
}

func compare(t *testing.T, expr, expectedExpr string, expectErr bool) {
	val, err := parser.ParseToPostfix(expr)
	if err != nil && !expectErr {
		t.Errorf("error got '%s'", err)
	}
	if val != expectedExpr {
		t.Errorf("value is not '%s', got '%s'", expectedExpr, val)
	}
}
