package agent

import (
	"context"
	"log"
	"strings"

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

	// seq tracks the global bubble index across all tool-loop iterations.
	var seq int32
	// allBubbles accumulates every bubble across iterations for DB persistence.
	var allBubbles []string

	for {
		turn, err := a.brain.Chat(ctx, a.system, hist, a.tools.Defs())
		if err != nil {
			return convID, err
		}

		for _, bubble := range turn.Bubbles {
			if err := sink.Bubble(seq, bubble); err != nil {
				return convID, err
			}
			seq++
		}
		allBubbles = append(allBubbles, turn.Bubbles...)

		if len(turn.Uses) == 0 {
			break
		}

		hist = append(hist, llm.Message{
			Role:     "assistant",
			Text:     strings.Join(turn.Bubbles, "\n\n"),
			ToolUses: turn.Uses,
		})

		toolResults := make([]llm.ToolResult, 0, len(turn.Uses))
		for _, use := range turn.Uses {
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

	asstMsg := &mneme.Message{
		ConversationId: convID,
		ProsoponId:     celineProsoponId,
		Content:        strings.Join(allBubbles, "\n\n"),
	}
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
