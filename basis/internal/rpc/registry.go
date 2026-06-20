package rpc

import (
	"strings"
	"sync"
)

// registry maps an authenticated sub to its live connection.
type registry struct {
	mu    sync.Mutex
	sigao map[string]chan struct{}
	pempo map[string]chan string
}

// Register installs a fresh connection for sub and reports true, but only if sub
// has no live session — first wins, at most one active session per user. It
// returns false when one already exists; the caller must refuse the new stream.
func (r *registry) Register(sub string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, f := r.pempo[sub]; f {
		return false
	}
	r.pempo[sub] = make(chan string, 64)
	r.sigao[sub] = make(chan struct{}, 1)
	return true
}

// Unregister removes sub's connection. Safe under first-wins (Register): while a
// session's entry is present, no successor can be installed, so the exiting loop
// only ever clears its own registration.
func (r *registry) Unregister(sub string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, found := r.pempo[sub]; found {
		delete(r.pempo, sub)
	}
	if _, found := r.sigao[sub]; found {
		delete(r.sigao, sub)
	}
}

func (r *registry) Sigao(sub string) (chan struct{}, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, f := r.sigao[sub]
	return c, f
}

func (r *registry) Pempo(sub string) (chan string, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, f := r.pempo[sub]
	return c, f
}

// messages accumulates queued user texts between flushes. Written by Pempo
// handlers, drained by the Parousia loop — hence the mutex.
type messages struct {
	items []string
}

func (m *messages) Enqueue(text string) {
	m.items = append(m.items, text)
}

// Flush joins everything queued into one combined user turn and clears the queue.
func (m *messages) Flush() string {
	combined := strings.TrimSpace(strings.Join(m.items, "\n"))
	m.items = make([]string, 0)
	return combined
}
