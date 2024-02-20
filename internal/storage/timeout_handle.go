package storage

import (
	"encoding/json"
	"fmt"
	"net/http"

	op "github.com/XJIeI5/calculator/internal/operation"
)

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
