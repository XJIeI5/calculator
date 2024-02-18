package storage

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"unicode"

	op "github.com/XJIeI5/calculator/internal/operation"
	"github.com/XJIeI5/calculator/internal/parser"
	"github.com/informitas/stack"
)

func (s *storage) calculateInParallel(addrCompServer string, expr postfixExpr) (<-chan error, <-chan float32) {

	errs := make(chan error)
	out := make(chan float32, 1)
	go func(expr string) {
		defer close(errs)
		defer close(out)
		locals := stack.NewStack[<-chan float32]()
		var skip int
		for i, r := range expr {
			if skip > 0 {
				skip--
				continue
			}
			if r == ' ' {
				continue
			}
			if unicode.IsDigit(r) {
				num := parser.GetStringNumber(expr[i:])
				v, _ := strconv.ParseFloat(num, 32)
				skip = len(num) - 1

				ch := make(chan float32, 1)
				ch <- float32(v)
				locals.Push(ch)
			} else {
				operand := parser.GetOperand(expr[i:])
				if operand == nil {
					errs <- fmt.Errorf("unknown operand '%s'", operand.Symbol())
					return
				}
				skip = len(operand.Symbol()) - 1
				if duration, ok := s.timeouts[string(r)]; ok {
					second, _ := locals.Pop()
					first, _ := locals.Pop()
					info := op.BinaryOperationInfo{A: <-first, B: <-second, Op: operand.Symbol()}
					res, err := calculateBinary(addrCompServer, duration, info)
					if err != nil {
						errs <- err
						return
					}
					ch := make(chan float32, 1)
					ch <- res
					locals.Push(ch)
				} else {
					errs <- fmt.Errorf("no timeout for '%c'", r)
					return
				}
			}
		}
		res, _ := locals.Pop()
		out <- <-res
	}(string(expr))
	return errs, out
}

func calculateBinary(addrComp string, dur int, binInfo op.BinaryOperationInfo) (float32, error) {
	data := struct {
		Dur                    int `json:"duration"`
		op.BinaryOperationInfo `json:"op_info"`
	}{Dur: dur, BinaryOperationInfo: binInfo}
	byteData, err := json.Marshal(data)
	if err != nil {
		return 0, err
	}

	resp, err := http.Post(fmt.Sprintf("%s/%s", addrComp, "exec"), "application/json", bytes.NewBuffer(byteData))
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	res, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	value, err := strconv.ParseFloat(string(res), 32)
	return float32(value), err
}
