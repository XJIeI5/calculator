package storage

import (
	"bytes"
	"crypto/sha1"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"

	datastructs "github.com/XJIeI5/calculator/internal/datastructs"
	"github.com/gorilla/mux"
)

// TODO: remove expr struct from exprQueue
type storage struct {
	db *sql.DB

	router             *mux.Router
	computationServers map[string]int64

	exprQueue *datastructs.Queue[expr]
	addr      string

	mu sync.RWMutex
}

func newStorage(db *sql.DB, addr string) *storage {
	// get not done expressions
	expressions, err := getInProcessExpressions(db)
	if err != nil && err != sql.ErrNoRows {
		panic(err)
	}
	// get stored computes
	computes, err := getComputes(db)
	if err != nil && err != sql.ErrNoRows {
		panic(err)
	}

	s := &storage{
		addr:               addr,
		db:                 db,
		computationServers: make(map[string]int64, 0),
		exprQueue:          datastructs.NewQueue[expr](10),
	}
	storeTimeout(s.db, "wait", 10000, 0)

	// set not done expressions queue
	for _, expr := range expressions {
		s.exprQueue.Enqueue(expr)
	}
	// sets computes
	for addr := range computes {
		data, err := json.Marshal(struct {
			Addr string `json:"addr"`
		}{Addr: addr})
		if err != nil {
			panic(err)
		}
		http.Post(fmt.Sprintf("%s/regist_compute", s.addr), "application/json", bytes.NewBuffer(data))
	}
	s.computationServers = computes
	fmt.Println(s.computationServers)
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
		Handler: newStorage(db, addr),
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
	key []byte
)

type expressionState struct {
	State  state       `json:"state"`
	Result interface{} `json:"result"`
}

type expr struct {
	postfixExpr
	userId int
}
