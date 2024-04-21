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

func (s *storage) handleAddExpression(w http.ResponseWriter, r *http.Request) {
	if t := r.Header.Get("Content-Type"); t != "application/json" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if t := r.Header.Get("Authorization"); t == "" {
		http.Error(w, "unknown user", http.StatusBadRequest)
		return
	}
	if len(s.computationServers) == 0 {
		http.Error(w, "no computation servers registered", http.StatusBadRequest)
		return
	}

	_expr := struct {
		Value string `json:"expr"`
	}{}

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	err := decoder.Decode(&_expr)

	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	parsedExpr, err := parser.ParseToPostfix(_expr.Value)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	hash := getHash(parsedExpr)
	bearerToken := r.Header.Get("Authorization")

	if id, err := checkExpressionExists(s.db, hash, bearerToken); err == nil {
		w.Write([]byte(strconv.FormatInt(id, 10)))
		fmt.Println("again")
		return
	}
	go func() {
		userId, err := getUserId(bearerToken)
		if err != nil {
			panic(err)
		}
		s.exprQueue.Enqueue(expr{postfixExpr: postfixExpr(parsedExpr), userId: userId})
	}()

	id, err := storeExpressionState(s.db, in_progress, nil, bearerToken, postfixExpr(parsedExpr))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write([]byte(strconv.FormatInt(int64(id), 10)))
}

func (s *storage) handleGetResult(w http.ResponseWriter, r *http.Request) {
	strId := r.URL.Query().Get("id")
	id, err := strconv.Atoi(strId)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	st, err := getExpressionState(s.db, id)
	if err != nil {
		http.Error(w, fmt.Sprintf("no expr with id %d", id), http.StatusBadRequest)
		return
	}
	data, err := json.Marshal(st)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write(data)
}

func (s *storage) calcExpressions() {
	for {
		_expr, err := s.exprQueue.Dequeue()
		if err != nil {
			continue
		}
		// waiting for expression then start calculation
		go func() {
			hashSum := getHash(string(_expr.postfixExpr))
			compAddr, err := s.getMostFreeComputationServer()
			if err != nil {
				updateExpressionState(s.db, has_error, err.Error(), hashSum)
				s.exprQueue.Enqueue(_expr)
				return
			}
			fmt.Println(compAddr)
			errs, res := s.calculateInSync(compAddr, _expr.postfixExpr, _expr.userId)
			for err := range errs {
				if err != nil {
					updateExpressionState(s.db, has_error, err.Error(), hashSum)
					return
				}
			}

			result := <-res
			updateExpressionState(s.db, ok, result, hashSum)
		}()
	}
}

func (s *storage) calculateInSync(addrCompServer string, expr postfixExpr, userId int) (<-chan error, <-chan float32) {
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
					errs <- fmt.Errorf("unknown operand")
					return
				}
				skip = len(operand.Symbol()) - 1
				duration, err := getOperandTime(s.db, string(r), userId)
				if err == nil {
					second, _ := locals.Pop()
					first, _ := locals.Pop()
					info := op.BinaryOperationInfo{A: <-first, B: <-second, Op: operand.Symbol()}
					res, err := calculateBinary(addrCompServer, int(duration.Milliseconds()), info)
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
