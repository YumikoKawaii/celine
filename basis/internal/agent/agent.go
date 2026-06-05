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

type Agent struct {
	brain  brain
	system string
	convs  *mneme.ConversationStore
	msgs   *mneme.MessageStore
}

func New(b brain, systemPrompt string, convs *mneme.ConversationStore, msgs *mneme.MessageStore) *Agent {
	return &Agent{brain: b, system: systemPrompt, convs: convs, msgs: msgs}
}

func (a *Agent) Chat(ctx context.Context, ownerSub, convID, userText string, sink EventSink) (string, error) {
	convID, err := a.convs.GetOrCreate(ctx, ownerSub, convID)
	if err != nil {
		return "", err
	}

	// Load history from Postgres to build Claude's message context.
	stored, err := a.msgs.GetHistory(ctx, convID, ownerSub)
	if err != nil {
		return "", err
	}
	hist := make([]llm.Message, 0, len(stored)+1)
	for _, m := range stored {
		hist = append(hist, llm.Message{Role: m.Role, Text: m.Content})
	}
	hist = append(hist, llm.Message{Role: "user", Text: userText})

	// Persist user message before streaming — durable even if the stream fails.
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

	// Enqueue both messages for vector indexing. Non-blocking — a failure here
	// doesn't break the chat; graphe will process whatever lands in the queue.
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
