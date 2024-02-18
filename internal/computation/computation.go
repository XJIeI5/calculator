package computation

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	op "github.com/XJIeI5/calculator/internal/operation"
	"github.com/XJIeI5/calculator/internal/parser"
	"github.com/gorilla/mux"
)

func GetServer(addr string, port, maxGoroutines int) *http.Server {
	var (
		_addr string
	)
	if strings.Contains(addr, "localhost") || strings.Contains(addr, "127.0.0.1") {
		_addr = fmt.Sprintf(":%d", port)
	} else {
		_addr = fmt.Sprintf("%s:%d", addr, port)
	}
	return &http.Server{
		Addr:    _addr,
		Handler: newComputationServer(int32(maxGoroutines), fmt.Sprintf("%s:%d", addr, port)),
	}
	// return &ComputationServer{maxGoroutines: maxGoroutines}
}

func newComputationServer(maxGoroutines int32, addr string) *computationServer {
	cs := &computationServer{
		maxGoroutines: maxGoroutines,
		addr:          addr,
	}
	r := mux.NewRouter()
	r.HandleFunc("/exec", cs.handleExec).Methods("POST")
	r.HandleFunc("/regist", cs.handleRegist).Methods("POST")
	r.HandleFunc("/free_process", cs.handleFreeProccesses).Methods("GET")
	cs.router = r
	return cs
}

type computationServer struct {
	addr              string
	storageAddr       string
	router            *mux.Router
	maxGoroutines     int32
	currentGoroutines int32
}

func (c *computationServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c.router.ServeHTTP(w, r)
}

func (c *computationServer) handleExec(w http.ResponseWriter, r *http.Request) {
	if atomic.LoadInt32(&c.currentGoroutines) >= int32(c.maxGoroutines) {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	atomic.AddInt32(&c.currentGoroutines, 1)

	defer atomic.AddInt32(&c.currentGoroutines, -1)
	execInfo := struct {
		op.BinaryOperationInfo `json:"op_info"`
		Duration               int `json:"duration"`
	}{}
	err := json.NewDecoder(r.Body).Decode(&execInfo)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	timer := time.NewTimer(time.Millisecond * time.Duration(execInfo.Duration))
	select {
	case <-timer.C:
		operand := parser.GetOperand(execInfo.Op)
		if operand == nil {
			http.Error(w, fmt.Sprintf("operand '%s' doesn't exist", execInfo.Op), http.StatusBadRequest)
			return
		}
		bin, ok := operand.(op.BinaryOperand)
		if !ok {
			http.Error(w, fmt.Sprintf("operand '%s' is not binary", operand.Symbol()), http.StatusBadRequest)
			return
		}
		res, err := bin.Exec(float64(execInfo.A), float64(execInfo.B))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Write([]byte(strconv.FormatFloat(float64(res), 'f', -1, 32)))
	}
}

func (c *computationServer) handleFreeProccesses(w http.ResponseWriter, r *http.Request) {
	cur := atomic.LoadInt32(&c.currentGoroutines)
	w.Write([]byte(fmt.Sprint(c.maxGoroutines - cur)))
}

func (c *computationServer) handleRegist(w http.ResponseWriter, r *http.Request) {
	if t := r.Header.Get("Content-Type"); t != "application/json" {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	type registerData struct {
		Addr string `json:"addr"`
	}

	storageData := registerData{}
	err := json.NewDecoder(r.Body).Decode(&storageData)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	autoGenerated := registerData{c.addr}
	data, err := json.Marshal(autoGenerated)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	resp, err := http.Post(fmt.Sprintf("%s/regist_compute", storageData.Addr), "application/json", bytes.NewBuffer(data))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()
	w.WriteHeader(resp.StatusCode)
	c.storageAddr = storageData.Addr
	go c.beat()
}

func (c *computationServer) beat() {
	ticker := time.NewTicker(time.Second * 1)
loop:
	for {
		select {
		case <-ticker.C:
			data := struct {
				Addr string `json:"addr"`
			}{
				Addr: c.addr,
			}
			b, _ := json.Marshal(data)
			resp, err := http.Post(fmt.Sprintf("%s/heart", c.storageAddr), "application/json", bytes.NewBuffer(b))
			if resp.StatusCode != http.StatusOK || err != nil {
				c.storageAddr = ""
				ticker.Stop()
				break loop
			}
		}
	}
}
