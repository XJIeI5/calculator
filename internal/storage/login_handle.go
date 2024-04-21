package storage

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/dgrijalva/jwt-go"
	"golang.org/x/crypto/bcrypt"
)

type registerUser struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

func (s *storage) handleRegister(w http.ResponseWriter, r *http.Request) {
	register := registerUser{}
	err := json.NewDecoder(r.Body).Decode(&register)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	q := `
	INSERT INTO users (login, hashedPassword) VALUES ($1, $2)
	`
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(register.Password), 14)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	res, err := s.db.Exec(q, register.Login, hashedPassword)
	if err != nil {
		panic(err)
	}
	timeouts := map[string]int{"+": 500, "*": 500, "/": 500, "-": 500}
	for operand, value := range timeouts {
		userId, err := res.LastInsertId()
		if err != nil {
			panic(err)
		}
		storeTimeout(s.db, operand, value, int(userId))
	}
}

func (s *storage) handleLogin(w http.ResponseWriter, r *http.Request) {
	if t := r.Header.Get("Content-Type"); t != "application/json" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	register := registerUser{}
	err := json.NewDecoder(r.Body).Decode(&register)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	q := `
	SELECT hashedPassword, id FROM users WHERE login = $1
	`
	rows, err := s.db.Query(q, register.Login)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var (
		hashedPassword string
		id             int
	)
	rows.Next()
	if err := rows.Scan(&hashedPassword, &id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(register.Password)); err != nil {
		http.Error(w, "incorrect password", http.StatusBadRequest)
		return
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"id":  strconv.Itoa(id),
		"nbf": time.Now().Unix(),
		"exp": time.Now().Add(30 * 24 * time.Hour).Unix(),
		"iat": time.Now().Unix(),
	})
	tokenString, err := token.SignedString(key)

	if err != nil {
		panic(err)
	}
	json.NewEncoder(w).Encode(tokenString)
}
