package storage

import (
	"crypto/sha1"
	"database/sql"
	"encoding/binary"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	datastructs "github.com/XJIeI5/calculator/internal/datastructs"
	"github.com/gorilla/mux"
)

// TODO: remove expr struct from exprQueue
type storage struct {
	db *sql.DB

	router             *mux.Router
	computationServers map[string]time.Time
	timeouts           map[string]int

	exprQueue *datastructs.Queue[postfixExpr]

	mu sync.RWMutex
}

func getInProcessExpressions(db *sql.DB) ([]postfixExpr, error) {
	var q string = `
	SELECT postfixExpression FROM expressions WHERE status = $1
	`
	rows, err := db.Query(q, in_progress)
	if err != nil {
		return []postfixExpr{}, err
	}
	expressions := make([]postfixExpr, 0)
	for rows.Next() {
		var expr string
		if err := rows.Scan(&expr); err != nil {
			return []postfixExpr{}, err
		}
		expressions = append(expressions, postfixExpr(expr))
	}
	return expressions, nil
}

func newStorage(db *sql.DB) *storage {
	expressions, err := getInProcessExpressions(db)
	if err != nil && err != sql.ErrNoRows {
		panic(err)
	}

	s := &storage{
		db:                 db,
		computationServers: make(map[string]time.Time, 0),
		timeouts:           map[string]int{"+": 500, "*": 500, "/": 500, "-": 500, "__wait": 10000},
		exprQueue:          datastructs.NewQueue[postfixExpr](10),
	}

	for _, expr := range expressions {
		s.exprQueue.Enqueue(expr)
	}
	// background processes
	go s.calcExpressions()
	go s.cleanComputationServers()

	r := mux.NewRouter()
	// expr handle
	r.HandleFunc("/add_expr", s.handleAddExpression).Methods("POST")
	r.HandleFunc("/get_result", s.handleGetResult).Methods("GET")
	// compute handle
	r.HandleFunc("/regist_compute", s.handleRegistCompute).Methods("POST")
	r.HandleFunc("/heart", s.handleHeartbeat).Methods("POST")
	r.HandleFunc("/get_compute", s.handleGetCompute).Methods("GET")
	// timeout handle
	r.HandleFunc("/set_timeout", s.handleSetTimeouts).Methods("POST")
	// login
	r.HandleFunc("/regist_user", s.handleRegister).Methods("POST")
	r.HandleFunc("/login", s.handleLogin).Methods("POST")

	s.router = r

	return s
}

func (s *storage) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

func GetServer(addr string, port int, db *sql.DB) *http.Server {
	var _addr string
	if strings.Contains(addr, "localhost") || strings.Contains(addr, "127.0.0.1") {
		_addr = fmt.Sprintf(":%d", port)
	} else {
		_addr = fmt.Sprintf("%s:%d", addr, port)
	}
	return &http.Server{
		Addr:    _addr,
		Handler: newStorage(db),
	}
}

type state string
type postfixExpr string
type exprHash int

func getHash(line string) exprHash {
	h := sha1.New()
	h.Write([]byte(line))
	return exprHash(binary.BigEndian.Uint32(h.Sum(nil)))
}

const (
	_           state = ""
	has_error   state = "error"
	in_progress state = "in progress"
	ok          state = "ok"
)

var (
	key []byte = []byte("secret")
)

type expressionState struct {
	State  state       `json:"state"`
	Result interface{} `json:"result"`
}
