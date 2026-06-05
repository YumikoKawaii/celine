package agent

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sync"

	"github.com/YumikoKawaii/celine/basis/internal/llm"
)

type EventSink interface {
	Typing(msHint int32) error
	Bubble(seq int32, text string) error
}

type brain interface {
	StreamChat(ctx context.Context, system string, history []llm.Message, deltas chan<- string) (string, error)
}

type Agent struct {
	brain  brain
	system string

	mu      sync.Mutex
	history map[string][]llm.Message
}

func New(b brain, systemPrompt string) *Agent {
	return &Agent{brain: b, system: systemPrompt, history: map[string][]llm.Message{}}
}

func (a *Agent) Chat(ctx context.Context, convID, userText string, sink EventSink) (string, error) {
	if convID == "" {
		convID = newConvID()
	}

	a.mu.Lock()
	hist := append([]llm.Message(nil), a.history[convID]...)
	a.mu.Unlock()
	hist = append(hist, llm.Message{Role: "user", Text: userText})

	deltas := make(chan string, 64)
	type result struct {
		text string
		err  error
	}
	done := make(chan result, 1)
	go func() {
		text, err := a.brain.StreamChat(ctx, a.system, hist, deltas)
		done <- result{text, err}
	}()

	if err := paceBubbles(ctx, deltas, sink); err != nil {
		return convID, err
	}
	res := <-done
	if res.err != nil {
		return convID, res.err
	}

	a.mu.Lock()
	a.history[convID] = append(a.history[convID],
		llm.Message{Role: "user", Text: userText},
		llm.Message{Role: "assistant", Text: res.text},
	)
	a.mu.Unlock()

	return convID, nil
}

func newConvID() string {
	var b [12]byte
	_, _ = rand.Read(b[:])
	return "conv-" + hex.EncodeToString(b[:])
}
