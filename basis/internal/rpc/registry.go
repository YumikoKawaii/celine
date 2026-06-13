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

// Register installs a fresh connection for sub, replacing any previous one —
// last writer wins, so a new tab supersedes the old.
func (r *registry) Register(sub string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pempo[sub] = make(chan string, 64)
	r.sigao[sub] = make(chan struct{}, 1)
}

// Unregister removes sub's connection only if it is still c — an old Parousia
// loop exiting must not tear down its successor's registration.
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
