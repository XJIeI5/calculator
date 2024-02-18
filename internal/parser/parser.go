package parser

import (
	"bufio"
	"fmt"
	"strconv"
	"strings"
	"unicode"

	op "github.com/XJIeI5/calculator/internal/operation"
	"github.com/informitas/stack"
)

var (
	errorNoChanges         error = fmt.Errorf("no changes")
	errorNotAllNumbersUsed       = fmt.Errorf("not all numbers are involved in mathematical operations")
	errorNotClosedParen          = fmt.Errorf("paren doesn't closed")
	errorNoOpenParen             = fmt.Errorf("closed paren located before open paren")
)

func GetStringNumber(expr string) (res string) {
	var (
		result string
	)

	sc := bufio.NewScanner(strings.NewReader(expr))
	sc.Split(bufio.ScanRunes)
	for sc.Scan() {
		if _, err := strconv.Atoi(sc.Text()); err == nil || sc.Text() == "." {
			result += sc.Text()
			continue
		}
		break
	}

	return result
}

func GetOperand(expr string) op.Operand {
	sc := bufio.NewScanner(strings.NewReader(expr))
	sc.Split(bufio.ScanRunes)
	var sym string
	for sc.Scan() {
		if sc.Text() == " " {
			break
		}
		if _, err := strconv.Atoi(sc.Text()); err == nil {
			break
		}
		sym += sc.Text()
	}
	for _, oper := range op.Operands {
		if sym == oper.Symbol() {
			return oper
		}
	}
	return nil
}

func addTo(res, parsed string) string {
	return fmt.Sprintf("%s%s ", res, parsed[:len(parsed)-1])
}

func ParseToPostfix(infixExpr string) (string, error) {
	var (
		res            string
		digitsInAction int
		skip           int
	)
	s := stack.NewStack[op.Operand]()

	consumeAll := func() (string, error) {
		var consumed string
		for !s.IsEmpty() {
			oper, _ := s.Pop()
			if _, ok := oper.(op.OrderOperand); ok {
				return consumed, errorNotClosedParen
			}
			if _, ok := oper.(op.BinaryOperand); ok {
				digitsInAction--
			}
			consumed = fmt.Sprintf("%s%s ", consumed, oper.Symbol())
		}
		return consumed, nil
	}

	for i, r := range infixExpr {
		if skip > 0 {
			skip--
			continue
		}

		if r == ' ' {
			continue
		}
		if unicode.IsDigit(r) { // PARSE DIGIT
			num := GetStringNumber(infixExpr[i:])
			res = addTo(res, num+" ")
			skip = len(num) - 1
			digitsInAction++
		} else { // PARSE OPERAND
			operand := GetOperand(infixExpr[i:])
			if operand == nil {
				return "", fmt.Errorf("unknown operand %c", r)
			}
			skip = len(operand.Symbol()) - 1
			switch t := operand.(type) {
			case op.MathOperand:
				parsedOpers, err := parseMathOperand(operand, s)
				if err == errorNoChanges {
					continue
				}
				if err != nil {
					return "", err
				}
				res = fmt.Sprintf("%s%s ", res, parsedOpers)
				if _, ok := operand.(op.BinaryOperand); ok {
					digitsInAction--
				}
			case op.OrderOperand:
				if t.IsStart() {
					s.Push(t)
				} else {
					if s.Size() <= 0 {
						return "", errorNoOpenParen
					}
					consumed, err := consumeAll()
					if err != nil && err != errorNotClosedParen {
						return "", err
					}
					if err != errorNotClosedParen {
						return "", errorNoOpenParen
					}
					res = addTo(res, consumed)
				}
			}
		}
	}
	consumed, err := consumeAll()
	if err != nil {
		return "", err
	}
	res = addTo(res, consumed)

	if digitsInAction != 1 {
		return "", errorNotAllNumbersUsed
	}

	return res, nil
}

func parseMathOperand(operand op.Operand, operStack *stack.Stack[op.Operand]) (string, error) {
	switch t := operand.(type) {
	case op.BinaryOperand:
		return parseBinaryOperand(t, operStack)
	case op.PostfixOperand:
		return parsePostfixOperand(t, operStack)
	case op.PrefixOperand:
		return parsePrefixOperand(t, operStack)
	default:
		return "", fmt.Errorf("operand '%s' is not defined", operand.Symbol())
	}
}

func parseBinaryOperand(operand op.BinaryOperand, operStack *stack.Stack[op.Operand]) (string, error) {
	var operands string

	for {
		peek, _ := operStack.Top()
		if operStack.Size() > 0 && op.OperationPriority[peek] >= op.OperationPriority[operand] {
			oper, _ := operStack.Pop()
			operands = fmt.Sprintf("%s%s ", operands, oper.Symbol())
			continue
		}
		break
	}

	// operands := getOperandsWithHigherPriority(operStack, op.OperationPriority[operand])
	operStack.Push(operand)
	if len(operands) != 0 {
		return operands[:len(operands)-1], nil
	}
	return "", errorNoChanges
}

func parsePostfixOperand(operand op.PostfixOperand, operStack *stack.Stack[op.Operand]) (string, error) {
	return "", fmt.Errorf("postfix operand parsing not implemented")
}

func parsePrefixOperand(operand op.PrefixOperand, operStack *stack.Stack[op.Operand]) (string, error) {
	return "", fmt.Errorf("prefix operand parsing not implemented")
}
