package storage

import (
	"database/sql"
	"strconv"
	"time"

	"github.com/dgrijalva/jwt-go"
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

func getWaitTime(db *sql.DB) (time.Duration, error) {
	var (
		q string = `
		SELECT value FROM timeouts WHERE type = $1 AND userId = $2
		`
		res int
	)
	err := db.QueryRow(q, "wait", 0).Scan(&res)
	if err != nil {
		return 0, err
	}
	return time.Duration(res) * time.Millisecond, nil
}

func getOperandTime(db *sql.DB, operand string, userId int) (time.Duration, error) {
	var (
		q string = `
		SELECT value FROM timeouts WHERE type = $1 AND userId = $2
		`
		res int
	)
	err := db.QueryRow(q, operand, userId).Scan(&res)
	if err != nil {
		return 0, err
	}
	return time.Duration(res) * time.Millisecond, nil
}

func storeTimeout(db *sql.DB, operand string, value int, userId int) error {
	var (
		q string = `
		SELECT id FROM timeouts WHERE type = $1 AND userId = $2
		`
		id int
	)

	if err := db.QueryRow(q, operand, userId).Scan(&id); err != nil {
		q = `
		INSERT INTO timeouts (type, value, userId) VALUES ($1, $2, $3)
		`
		if _, err := db.Exec(q, operand, value, userId); err != nil {
			return err
		}
	} else {
		q = `
		UPDATE timeouts SET value = $1 WHERE id = $2 AND type = $3`
		if _, err := db.Exec(q, value, id, operand); err != nil {
			return err
		}
	}
	return nil
}

func getComputes(db *sql.DB) (map[string]int64, error) {
	var q string = `
	SELECT address, lastPing FROM computes
	`
	res := make(map[string]int64)
	rows, err := db.Query(q)
	if err != nil {
		return res, err
	}
	for rows.Next() {
		var (
			addr string
			ping int64
		)
		if err := rows.Scan(&addr, &ping); err != nil {
			return res, err
		}
		res[addr] = ping
	}
	return res, nil
}

func deleteCompute(db *sql.DB, addr string) error {
	var q string = `
	DELETE FROM computes WHERE address = $1`
	_, err := db.Exec(q, addr)
	return err
}

func storeCompute(db *sql.DB, addr string, lastPing int64) error {
	var q string = `
	INSERT INTO computes (address, lastPing) VALUES ($1, $2)
	`
	_, err := db.Exec(q, addr, lastPing)
	return err
}

func pingCompute(db *sql.DB, addr string, lastPing int64) error {
	var q string = `
	UPDATE computes SET lastPing = $1 WHERE address = $2
	`
	_, err := db.Exec(q, lastPing, addr)
	return err
}
