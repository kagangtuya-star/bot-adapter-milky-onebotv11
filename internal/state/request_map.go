package state

import (
	"fmt"
	"sync"
	"sync/atomic"

	"milky-onebot11-bridge/internal/types"
)

type RequestMap struct {
	mu      sync.RWMutex
	counter atomic.Uint64
	items   map[string]types.RequestRef
}

func NewRequestMap() *RequestMap {
	return &RequestMap{items: make(map[string]types.RequestRef)}
}

func (r *RequestMap) Put(ref types.RequestRef) string {
	flag := fmt.Sprintf("%s:%d", ref.Kind, r.counter.Add(1))
	r.mu.Lock()
	defer r.mu.Unlock()
	r.items[flag] = ref
	return flag
}

func (r *RequestMap) Get(flag string) (types.RequestRef, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ref, ok := r.items[flag]
	return ref, ok
}
