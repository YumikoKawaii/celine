package rpc

import (
	"context"
	"strings"
	"sync"
	"time"

	celinev1 "github.com/YumikoKawaii/celine/basis/gen/celine/v1"
)

type session struct {
	ch       chan *celinev1.ParousiaEvent
	ctx      context.Context
	mu       sync.Mutex
	inbox    []string
	busy     bool
	timer    *time.Timer
	debounce time.Duration
	flush    func()
}

func (s *session) pempo(text string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.inbox = append(s.inbox, text)

	if s.busy {
		return
	}

	if s.timer != nil {
		s.timer.Reset(s.debounce)
	} else {
		s.timer = time.AfterFunc(s.debounce, s.flush)
	}
}

func (s *session) drainInbox() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	lines := s.inbox
	s.inbox = nil
	s.timer = nil
	s.busy = true
	return strings.Join(lines, "\n")
}

func (s *session) clearBusy() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.busy = false

	if len(s.inbox) > 0 {
		s.timer = time.AfterFunc(s.debounce, s.flush)
	}
}

type sessionStore struct {
	mu       sync.RWMutex
	sessions map[string]*session
}

func newSessionStore() *sessionStore {
	return &sessionStore{sessions: make(map[string]*session)}
}

func (s *sessionStore) register(sub string, ctx context.Context, debounce time.Duration, flush func(sub string)) *session {
	ch := make(chan *celinev1.ParousiaEvent, 64)
	sess := &session{
		ch:       ch,
		ctx:      ctx,
		debounce: debounce,
	}
	sess.flush = func() { flush(sub) }

	s.mu.Lock()
	s.sessions[sub] = sess
	s.mu.Unlock()
	return sess
}

func (s *sessionStore) unregister(sub string) {
	s.mu.Lock()
	if sess, ok := s.sessions[sub]; ok {
		sess.mu.Lock()
		if sess.timer != nil {
			sess.timer.Stop()
			sess.timer = nil
		}
		sess.mu.Unlock()
	}
	delete(s.sessions, sub)
	s.mu.Unlock()
}

func (s *sessionStore) get(sub string) (*session, bool) {
	s.mu.RLock()
	sess, ok := s.sessions[sub]
	s.mu.RUnlock()
	return sess, ok
}
