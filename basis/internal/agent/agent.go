package agent

import (
	"context"
	"log"

	"github.com/YumikoKawaii/celine/basis/internal/arche"
	"github.com/YumikoKawaii/celine/basis/internal/llm"
	"github.com/YumikoKawaii/celine/basis/internal/mneme"
)

// celineProsoponId aliases arche.CelineProsoponID for package-local readability.
const celineProsoponId = arche.CelineProsoponID

type Agent struct {
	brain         brain
	system        string
	prosopons     prosopons
	conversations conversations
	messages      messages
	queue         queue
	tools         toolRunner
}

func New(
	b brain,
	systemPrompt string,
	prosopons prosopons,
	conversations conversations,
	messages messages,
	q queue,
	tools toolRunner,
) *Agent {
	return &Agent{
		brain:         b,
		system:        systemPrompt,
		prosopons:     prosopons,
		conversations: conversations,
		messages:      messages,
		queue:         q,
		tools:         tools,
	}
}

func (a *Agent) Chat(ctx context.Context, ownerSub string, userText string, sink EventSink) (int64, error) {
	p, err := a.prosopons.Get(ctx, mneme.KataSub{Sub: ownerSub})
	if err != nil {
		return 0, err
	}

	conv, err := a.conversations.GetOrCreate(ctx, mneme.KataProsopon{ProsoponId: p.Id})
	if err != nil {
		return 0, err
	}
	convID := conv.Id

	stored, err := a.messages.List(ctx, historyMessages{convID: convID}, nil)
	if err != nil {
		return 0, err
	}
	hist := make([]llm.Message, 0, len(stored)+1)
	for _, m := range stored {
		role := "user"
		if m.ProsoponId == celineProsoponId {
			role = "assistant"
		}
		hist = append(hist, llm.Message{Role: role, Text: m.Content})
	}
	hist = append(hist, llm.Message{Role: "user", Text: userText})

	userMsg := &mneme.Message{ConversationId: convID, ProsoponId: p.Id, Content: userText}
	if err := a.messages.Create(ctx, userMsg); err != nil {
		return convID, err
	}
	a.enqueue(ctx, userMsg.Id)

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

		hist = append(hist, llm.Message{
			Role:     "assistant",
			Text:     res.turn.Text,
			ToolUses: res.turn.Uses,
		})

		toolResults := make([]llm.ToolResult, 0, len(res.turn.Uses))
		for _, use := range res.turn.Uses {
			_ = sink.ToolCall(use.Id, use.Name, string(use.Input))

			output, execErr := a.tools.Execute(ctx, use.Name, use.Input)
			isErr := execErr != nil
			if isErr {
				output = execErr.Error()
			}
			_ = sink.ToolResult(use.Id, output, isErr)

			toolResults = append(toolResults, llm.ToolResult{
				Id:      use.Id,
				Output:  output,
				IsError: isErr,
			})
		}

		hist = append(hist, llm.Message{Role: "user", ToolResults: toolResults})
	}

	asstMsg := &mneme.Message{ConversationId: convID, ProsoponId: celineProsoponId, Content: finalText}
	if err := a.messages.Create(ctx, asstMsg); err != nil {
		return convID, err
	}
	a.enqueue(ctx, asstMsg.Id)

	return convID, nil
}

func (a *Agent) enqueue(ctx context.Context, msgID int64) {
	if err := a.queue.Enqueue(ctx, arche.GrapheQueue, msgID); err != nil {
		log.Printf("agent: enqueue %d: %v", msgID, err)
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
