package status

import (
	"sync"
	"time"
)

type Component struct {
	Status string    `json:"status"`
	Last   time.Time `json:"-"`
	Error  string    `json:"-"`
}

type Snapshot struct {
	RouterOS Component
	AdGuard  Component
}

type Tracker struct {
	mu       sync.RWMutex
	routerOS Component
	adGuard  Component
}

func New() *Tracker {
	return &Tracker{
		routerOS: Component{Status: "unknown"},
		adGuard:  Component{Status: "unknown"},
	}
}

func (t *Tracker) RouterOSSuccess(at time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.routerOS = Component{Status: "ok", Last: at}
}

func (t *Tracker) RouterOSError(at time.Time, err error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.routerOS = Component{Status: "error", Last: at, Error: err.Error()}
}

func (t *Tracker) AdGuardSuccess(at time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.adGuard = Component{Status: "ok", Last: at}
}

func (t *Tracker) AdGuardError(at time.Time, err error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.adGuard = Component{Status: "error", Last: at, Error: err.Error()}
}

func (t *Tracker) Snapshot() Snapshot {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return Snapshot{RouterOS: t.routerOS, AdGuard: t.adGuard}
}
