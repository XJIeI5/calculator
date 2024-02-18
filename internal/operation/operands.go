package op

import "errors"

type Operand interface {
	Symbol() string
	Name() string
}

type MathOperand interface {
	Operand
	math()
}

type BinaryOperand interface {
	MathOperand
	Exec(a, b float64) (float64, error)
}

type UnaryOperand interface {
	MathOperand
	Exec(a float64) float64
}

type PostfixOperand interface {
	UnaryOperand
	postfix()
}

type PrefixOperand interface {
	UnaryOperand
	prefix()
}

type OrderOperand interface {
	Operand
	IsStart() bool
}

// ADD
type add struct{}

func (a add) math()          {}
func (a add) Symbol() string { return "+" }
func (a add) Name() string   { return "add" }

func (ad add) Exec(a, b float64) (float64, error) { return a + b, nil }

// SUB
type sub struct{}

func (s sub) math()          {}
func (s sub) Symbol() string { return "-" }
func (s sub) Name() string   { return "sub " }

func (s sub) Exec(a, b float64) (float64, error) { return a - b, nil }

// MULT
type mult struct{}

func (m mult) math()          {}
func (m mult) Symbol() string { return "*" }
func (m mult) Name() string   { return "mult" }

func (m mult) Exec(a, b float64) (float64, error) { return a * b, nil }

// DIV
type div struct{}

func (d div) math()          {}
func (d div) Symbol() string { return "/" }
func (d div) Name() string   { return "div" }

func (d div) Exec(a, b float64) (float64, error) {
	if b == 0 {
		return 0, errors.New("zero division")
	}
	return a / b, nil
}

// OPEN PAREN
type openParen struct{}

func (p openParen) Symbol() string { return "(" }
func (p openParen) Name() string   { return "open paren" }
func (p openParen) IsStart() bool  { return true }

// CLOSE PAREN
type closeParen struct{}

func (p closeParen) Symbol() string { return ")" }
func (p closeParen) Name() string   { return "close paren" }
func (p closeParen) IsStart() bool  { return false }
