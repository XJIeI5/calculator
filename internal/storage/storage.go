package storage

import (
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	datastructs "github.com/XJIeI5/calculator/internal/datastructs"
	"github.com/gorilla/mux"
)

type storage struct {
	router             *mux.Router
	computationServers map[string]time.Time
	timeouts           map[string]int
	expressions        *sync.Map

	exprQueue *datastructs.Queue[postfixExpr]

	mu sync.RWMutex
}

func newStorage() *storage {
	s := &storage{
		expressions:        &sync.Map{},
		computationServers: make(map[string]time.Time, 0),
		timeouts:           map[string]int{"+": 500, "*": 500, "/": 500, "-": 500, "__wait": 10000},
		exprQueue:          datastructs.NewQueue[postfixExpr](10),
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

	s.router = r

	return s
}

func (s *storage) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

func GetServer(addr string, port int) *http.Server {
	var _addr string
	if strings.Contains(addr, "localhost") || strings.Contains(addr, "127.0.0.1") {
		_addr = fmt.Sprintf(":%d", port)
	} else {
		_addr = fmt.Sprintf("%s:%d", addr, port)
	}
	return &http.Server{
		Addr:    _addr,
		Handler: newStorage(),
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

type expressionState struct {
	State  state       `json:"state"`
	Result interface{} `json:"result"`
}
