package storage

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strconv"
	"sync"
	"time"
)

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

func (s *storage) handleGetCompute(w http.ResponseWriter, r *http.Request) {
	type compState struct {
		Addr     string    `json:"addr"`
		State    string    `json:"state"`
		LastBeat time.Time `json:"last_beat"`
	}
	states := make([]compState, 0, len(s.computationServers))
	for _, addr := range s.getWorkingComputationServers() {
		st := compState{Addr: addr, LastBeat: s.computationServers[addr]}
		if time.Since(s.computationServers[addr]) > time.Duration(s.timeouts["__wait"])*time.Millisecond {
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
