package rpc

import (
	"context"
	"sync"

	celinev1 "github.com/YumikoKawaii/celine/basis/gen/celine/v1"
)

// session holds the live Parousia stream channel and the context tied to that
// stream's lifetime. When the client disconnects, ctx is cancelled — any agent
// goroutine spawned by Pempo that received this ctx will exit cleanly.
type session struct {
	ch  chan *celinev1.ParousiaEvent
	ctx context.Context
}

type sessionStore struct {
	mu       sync.RWMutex
	sessions map[string]session
}

func newSessionStore() *sessionStore {
	return &sessionStore{sessions: make(map[string]session)}
}

func (s *sessionStore) register(sub string, ctx context.Context) chan *celinev1.ParousiaEvent {
	ch := make(chan *celinev1.ParousiaEvent, 64)
	s.mu.Lock()
	s.sessions[sub] = session{ch: ch, ctx: ctx}
	s.mu.Unlock()
	return ch
}

func (s *sessionStore) unregister(sub string) {
	s.mu.Lock()
	delete(s.sessions, sub)
	s.mu.Unlock()
}

func (s *sessionStore) get(sub string) (session, bool) {
	s.mu.RLock()
	sess, ok := s.sessions[sub]
	s.mu.RUnlock()
	return sess, ok
}
