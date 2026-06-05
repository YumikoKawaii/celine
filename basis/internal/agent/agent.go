package agent

import (
	"context"
	"encoding/json"
	"log"

	"github.com/YumikoKawaii/celine/basis/internal/llm"
	"github.com/YumikoKawaii/celine/basis/internal/mneme"
)

// EventSink receives all events produced during a single Chat turn.
type EventSink interface {
	Typing(msHint int32) error
	Bubble(seq int32, text string) error
	ToolCall(id, name, inputJSON string) error
	ToolResult(id, output string, isError bool) error
}

type brain interface {
	StreamChat(ctx context.Context, system string, history []llm.Message, tools []llm.ToolDef, deltas chan<- string) (llm.Turn, error)
}

type convStore interface {
	GetOrCreate(ctx context.Context, ownerSub, convID string) (string, error)
}

type msgStore interface {
	Save(ctx context.Context, convID, role, content string) (string, error)
	GetHistory(ctx context.Context, convID, ownerSub string) ([]mneme.Message, error)
	Enqueue(ctx context.Context, job mneme.IndexJob) error
}

type toolRunner interface {
	Defs() []llm.ToolDef
	Execute(ctx context.Context, name string, input json.RawMessage) (string, error)
}

type Agent struct {
	brain  brain
	system string
	convs  convStore
	msgs   msgStore
	tools  toolRunner
}

func New(b brain, systemPrompt string, convs convStore, msgs msgStore, tools toolRunner) *Agent {
	return &Agent{brain: b, system: systemPrompt, convs: convs, msgs: msgs, tools: tools}
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
	a.enqueue(ctx, userMsgID, ownerSub, "user", userText)

	// countingSink remaps each per-segment bubble seq to a global turn-level
	// seq so indices stay unique across multiple StreamChat iterations.
	var seqOffset int32
	cs := &countingSink{inner: sink, seq: &seqOffset}

	var finalText string
	for {
		deltas := make(chan string, 64)
		type result struct {
			turn llm.Turn
			err  error
		}
		done := make(chan result, 1)
		go func() {
			turn, err := a.brain.StreamChat(ctx, a.system, hist, a.tools.Defs(), deltas)
			done <- result{turn, err}
		}()

		if err := paceBubbles(ctx, deltas, cs); err != nil {
			return convID, err
		}
		res := <-done
		if res.err != nil {
			return convID, res.err
		}

		if len(res.turn.Uses) == 0 {
			finalText = res.turn.Text
			break
		}

		// Append assistant turn (text + tool_use blocks) to in-memory history.
		hist = append(hist, llm.Message{
			Role:     "assistant",
			Text:     res.turn.Text,
			ToolUses: res.turn.Uses,
		})

		// Execute tools and feed results back.
		toolResults := make([]llm.ToolResult, 0, len(res.turn.Uses))
		for _, use := range res.turn.Uses {
			_ = sink.ToolCall(use.ID, use.Name, string(use.Input))

			output, execErr := a.tools.Execute(ctx, use.Name, use.Input)
			isErr := execErr != nil
			if isErr {
				output = execErr.Error()
			}
			_ = sink.ToolResult(use.ID, output, isErr)

			toolResults = append(toolResults, llm.ToolResult{
				ID:      use.ID,
				Output:  output,
				IsError: isErr,
			})
		}

		hist = append(hist, llm.Message{Role: "user", ToolResults: toolResults})
	}

	asstMsgID, err := a.msgs.Save(ctx, convID, "assistant", finalText)
	if err != nil {
		return convID, err
	}
	a.enqueue(ctx, asstMsgID, ownerSub, "assistant", finalText)

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

// countingSink remaps each bubble's local seq (reset per StreamChat call) to a
// monotonically increasing global seq across the whole turn.
type countingSink struct {
	inner EventSink
	seq   *int32
}

func (c *countingSink) Typing(ms int32) error { return c.inner.Typing(ms) }
func (c *countingSink) Bubble(_ int32, text string) error {
	s := *c.seq
	*c.seq++
	return c.inner.Bubble(s, text)
}
func (c *countingSink) ToolCall(id, name, input string) error {
	return c.inner.ToolCall(id, name, input)
}
func (c *countingSink) ToolResult(id, output string, isError bool) error {
	return c.inner.ToolResult(id, output, isError)
}
