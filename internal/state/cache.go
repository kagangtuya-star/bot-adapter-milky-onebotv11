package state

import (
	"sync"

	"milky-onebot11-bridge/internal/types"
)

type Runtime struct {
	mu                sync.RWMutex
	login             types.LoginInfo
	upstreamConnected bool
}

func NewRuntime() *Runtime {
	return &Runtime{}
}

func (r *Runtime) SetLogin(info types.LoginInfo) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.login = info
}

func (r *Runtime) Login() types.LoginInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.login
}

func (r *Runtime) SetUpstreamConnected(connected bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.upstreamConnected = connected
}

func (r *Runtime) Status() types.Status {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return types.Status{
		Online: r.upstreamConnected,
		Good:   r.upstreamConnected && r.login.SelfID != 0,
	}
}
