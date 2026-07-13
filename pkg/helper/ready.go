package helper

import (
	"net/http"
	"sync/atomic"
)

var Ready *Readiness

func init() {
	Ready = NewReadiness()
}

type Readiness struct {
	ready atomic.Bool
}

func NewReadiness() *Readiness {
	return &Readiness{}
}

func (r *Readiness) SetReady(ready bool) {
	r.ready.Store(ready)
}

func (r *Readiness) Handler(w http.ResponseWriter, req *http.Request) {
	if !r.ready.Load() {
		http.Error(w, "Not Ready\n", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("OK\n"))
}
