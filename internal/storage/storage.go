package storage

import (
	"crypto/sha1"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	datastructs "github.com/XJIeI5/calculator/internal/datastructs"
	op "github.com/XJIeI5/calculator/internal/operation"
	"github.com/XJIeI5/calculator/internal/parser"
	"github.com/gorilla/mux"
)

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

	go s.calcExpressions()
	go s.cleanComputationServers()

	r := mux.NewRouter()
	r.HandleFunc("/add_expr", s.handleAddExpression).Methods("POST")
	r.HandleFunc("/get_result", s.handleGetResult).Methods("GET")
	r.HandleFunc("/regist_compute", s.handleRegistCompute).Methods("POST")
	r.HandleFunc("/set_timeout", s.handleSetTimeouts).Methods("POST")
	r.HandleFunc("/heart", s.handleHeartbeat).Methods("POST")
	r.HandleFunc("/get_compute", s.handleGetCompute).Methods("GET")

	s.router = r

	return s
}

func (s *storage) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

func (s *storage) handleAddExpression(w http.ResponseWriter, r *http.Request) {
	if t := r.Header.Get("Content-Type"); t != "application/json" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if len(s.computationServers) == 0 {
		http.Error(w, "no computation servers registered", http.StatusBadRequest)
		return
	}

	expr := struct {
		Value string `json:"expr"`
	}{}

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	err := decoder.Decode(&expr)

	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	parsedExpr, err := parser.ParseToPostfix(expr.Value)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	id := getHash(parsedExpr)
	if _, ok := s.expressions.Load(id); ok {
		w.Write([]byte(strconv.FormatInt(int64(id), 10)))
		return
	}
	s.expressions.Store(id, &expressionState{State: in_progress, Result: nil})
	go s.exprQueue.Enqueue(postfixExpr(parsedExpr))
	w.Write([]byte(strconv.FormatInt(int64(id), 10)))
}

func (s *storage) calcExpressions() {
	for {
		expr, err := s.exprQueue.Dequeue()
		if err != nil {
			continue
		}
		go func() {
			hashSum := getHash(string(expr))
			compAddr, err := s.getMostFreeComputationServer()
			if err != nil {
				s.expressions.Store(hashSum, &expressionState{State: has_error, Result: err.Error()})
				return
			}
			fmt.Println(compAddr)
			errs, res := s.calculateInParallel(compAddr, expr)
			for err := range errs {
				if err != nil {
					s.expressions.Store(hashSum, &expressionState{State: has_error, Result: err.Error()})
					return
				}
			}

			s.expressions.Store(hashSum, &expressionState{State: ok, Result: <-res})
		}()
	}
}

func (s *storage) handleGetResult(w http.ResponseWriter, r *http.Request) {
	strId := r.URL.Query().Get("id")
	id, err := strconv.Atoi(strId)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	st, ok := s.expressions.Load(exprHash(id))
	if !ok {
		http.Error(w, fmt.Sprintf("no expr with id %d", id), http.StatusBadRequest)
	}
	data, err := json.Marshal(st)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write(data)
}

func (s *storage) getMostFreeComputationServer() (string, error) {
	var (
		wg          sync.WaitGroup
		mu          sync.Mutex
		freeProcess = make(map[string]int)
	)

	for _, addr := range s.getWorkingComputationServers() {
		wg.Add(1)
		go func(addr string) {
			defer wg.Done()
			resp, err := http.Get(fmt.Sprintf("%s/%s", addr, "free_process"))
			if err != nil {

				return
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				return
			}
			data, err := io.ReadAll(resp.Body)
			if err != nil {
				return
			}
			num, err := strconv.Atoi(string(data))
			if err != nil {
				return
			}
			mu.Lock()
			defer mu.Unlock()
			freeProcess[addr] = num
		}(addr)
	}
	wg.Wait()
	keys := make([]string, 0, len(freeProcess))
	for key := range freeProcess {
		if freeProcess[key] == 0 {
			continue
		}
		keys = append(keys, key)
	}
	if len(keys) == 0 {
		return "", fmt.Errorf("no availble computation server")
	}
	sort.Slice(keys, func(i, j int) bool { return freeProcess[keys[i]] > freeProcess[keys[j]] })
	return keys[0], nil
}

func (s *storage) getWorkingComputationServers() []string {
	res := make([]string, 0, len(s.computationServers))
	for addr := range s.computationServers {
		res = append(res, addr)
	}
	return res
}

func (s *storage) cleanComputationServers() {
	ticker := time.NewTicker(time.Second * 1)
	for range ticker.C {
		for addr, t := range s.computationServers {
			if time.Since(t) > time.Duration(s.timeouts["__wait"])*time.Millisecond {
				delete(s.computationServers, addr)
			}
		}
	}
}

func (s *storage) handleRegistCompute(w http.ResponseWriter, r *http.Request) {
	if t := r.Header.Get("Content-Type"); t != "application/json" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	registerData := struct {
		Addr string `json:"addr"`
	}{}

	err := json.NewDecoder(r.Body).Decode(&registerData)
	io.Copy(os.Stdout, r.Body)

	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	s.computationServers[registerData.Addr] = time.Now()
	w.WriteHeader(http.StatusOK)
}

func (s *storage) handleSetTimeouts(w http.ResponseWriter, r *http.Request) {
	if t := r.Header.Get("Content-Type"); t != "application/json" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	timeouts := struct {
		Value map[string]int `json:"timeout"`
	}{}
	err := json.NewDecoder(r.Body).Decode(&timeouts)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	for key, value := range timeouts.Value {
		if !op.HaveOperand(key) {
			http.Error(w, fmt.Sprintf("there is no operand '%s'", key), http.StatusBadRequest)
			return
		}
		s.timeouts[key] = value
	}
	w.WriteHeader(http.StatusOK)
}

func (s *storage) handleHeartbeat(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	auto := struct {
		Addr string `json:"addr"`
	}{}
	err := json.NewDecoder(r.Body).Decode(&auto)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	s.computationServers[auto.Addr] = time.Now()
	w.WriteHeader(http.StatusOK)
}

func (s *storage) handleGetCompute(w http.ResponseWriter, r *http.Request) {
	type compState struct {
		Addr     string    `json:"addr"`
		State    string    `json:"state"`
		LastBeat time.Time `json:"last_beat"`
	}
	states := make([]compState, 0, len(s.computationServers))
	for _, addr := range s.getWorkingComputationServers() {
		st := compState{Addr: addr, LastBeat: s.computationServers[addr]}
		if time.Since(s.computationServers[addr]) > 6*time.Second {
			st.State = "lost connection"
		} else {
			st.State = "available"
		}
		states = append(states, st)
	}

	data, err := json.Marshal(states)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write(data)
}
