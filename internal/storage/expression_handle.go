package storage

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"unicode"

	op "github.com/XJIeI5/calculator/internal/operation"
	"github.com/XJIeI5/calculator/internal/parser"
	"github.com/dgrijalva/jwt-go"
	"github.com/informitas/stack"
)

func validateToken(bearerToken string) (*jwt.Token, error) {
	tokenString := bearerToken
	token, err := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
		return key, nil
	})
	return token, err
}

func getUserId(bearerToken string) (int, error) {
	token, err := validateToken(bearerToken)
	if err != nil {
		return 0, err
	}
	if !token.Valid {
		return 0, err
	}

	user := token.Claims.(jwt.MapClaims)
	id, err := strconv.Atoi(user["id"].(string))
	if err != nil {
		return 0, err
	}
	return id, nil
}

func storeExpressionState(db *sql.DB, status state, result interface{}, bearerToken string, _expr postfixExpr) (int64, error) {
	var q string = `
	INSERT INTO expressions (status, result, userId, hash, postfixExpression) VALUES ($1, $2, $3, $4, $5)
	`

	id, err := getUserId(bearerToken)
	if err != nil {
		panic(err)
	}

	res, err := db.Exec(q, status, result, id, getHash(string(_expr)), _expr)
	if err != nil {
		panic(err)
	}
	return res.LastInsertId()
}

func updateExpressionState(db *sql.DB, status state, result interface{}, hash exprHash) {
	var q string = `
	UPDATE expressions SET status = $1, result = $2 WHERE hash = $4
	`

	if _, err := db.Exec(q, status, result, hash); err != nil {
		panic(err)
	}
}

func checkExpressionExists(db *sql.DB, hash exprHash, bearerToken string) (int64, error) {
	var q string = `
	SELECT id FROM expressions WHERE hash = $1 AND userId = $2
	`

	userId, err := getUserId(bearerToken)
	if err != nil {
		panic(err)
	}

	var id int64
	err = db.QueryRow(q, hash, userId).Scan(&id)
	return id, err
}

func getExpressionState(db *sql.DB, id int) (expressionState, error) {
	var q string = `
	SELECT status, result FROM expressions WHERE id = $1`
	var (
		st     state
		result string
	)
	if err := db.QueryRow(q, id).Scan(&st, &result); err != nil {
		return expressionState{}, err
	}
	return expressionState{State: st, Result: result}, nil
}

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
	go s.exprQueue.Enqueue(postfixExpr(parsedExpr))

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
			hashSum := getHash(string(_expr))
			compAddr, err := s.getMostFreeComputationServer()
			if err != nil {
				updateExpressionState(s.db, has_error, err.Error(), hashSum)
				s.exprQueue.Enqueue(_expr)
				return
			}
			fmt.Println(compAddr)
			errs, res := s.calculateInSync(compAddr, _expr)
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

func (s *storage) calculateInSync(addrCompServer string, expr postfixExpr) (<-chan error, <-chan float32) {
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
