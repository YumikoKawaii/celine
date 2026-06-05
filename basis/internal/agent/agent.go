package agent

import (
	"context"
	"log"

	"github.com/YumikoKawaii/celine/basis/internal/llm"
	"github.com/YumikoKawaii/celine/basis/internal/mneme"
)

type EventSink interface {
	Typing(msHint int32) error
	Bubble(seq int32, text string) error
}

type brain interface {
	StreamChat(ctx context.Context, system string, history []llm.Message, deltas chan<- string) (string, error)
}

// convStore is the subset of mneme.ConversationStore this package needs.
type convStore interface {
	GetOrCreate(ctx context.Context, ownerSub, convID string) (string, error)
}

// msgStore is the subset of mneme.MessageStore this package needs.
type msgStore interface {
	Save(ctx context.Context, convID, role, content string) (string, error)
	GetHistory(ctx context.Context, convID, ownerSub string) ([]mneme.Message, error)
	Enqueue(ctx context.Context, job mneme.IndexJob) error
}

type Agent struct {
	brain  brain
	system string
	convs  convStore
	msgs   msgStore
}

func New(b brain, systemPrompt string, convs convStore, msgs msgStore) *Agent {
	return &Agent{brain: b, system: systemPrompt, convs: convs, msgs: msgs}
}

func (a *Agent) Chat(ctx context.Context, ownerSub, convID, userText string, sink EventSink) (string, error) {
	convID, err := a.convs.GetOrCreate(ctx, ownerSub, convID)
	if err != nil {
		return "", err
	}

	stored, err := a.msgs.GetHistory(ctx, convID, ownerSub)
	if err != nil {
		return "", err
	}
	hist := make([]llm.Message, 0, len(stored)+1)
	for _, m := range stored {
		hist = append(hist, llm.Message{Role: m.Role, Text: m.Content})
	}
	hist = append(hist, llm.Message{Role: "user", Text: userText})

	userMsgID, err := a.msgs.Save(ctx, convID, "user", userText)
	if err != nil {
		return convID, err
	}

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

	asstMsgID, err := a.msgs.Save(ctx, convID, "assistant", res.text)
	if err != nil {
		return convID, err
	}

	a.enqueue(ctx, userMsgID, ownerSub, "user", userText)
	a.enqueue(ctx, asstMsgID, ownerSub, "assistant", res.text)

	return convID, nil
}

func (a *Agent) enqueue(ctx context.Context, msgID, ownerSub, role, content string) {
	if err := a.msgs.Enqueue(ctx, mneme.IndexJob{
		MessageID: msgID,
		OwnerSub:  ownerSub,
		Role:      role,
		Content:   content,
	}); err != nil {
		log.Printf("agent: enqueue %s: %v", msgID, err)
	}
}
