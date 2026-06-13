package agent

import (
	"context"
	"log"
	"strings"

	"github.com/YumikoKawaii/celine/basis/internal/arche"
	"github.com/YumikoKawaii/celine/basis/internal/ergon"
	"github.com/YumikoKawaii/celine/basis/internal/llm"
	"github.com/YumikoKawaii/celine/basis/internal/mneme"
)

// celineProsoponId aliases arche.CelineProsoponID for package-local readability.
const celineProsoponId = arche.CelineProsoponID

// Recall tuning for the §12.5 tier-2 thresholded hint.
const (
	// defaultRecallK is how many memories the auto-hint search pulls.
	defaultRecallK = 5
	// defaultRecallThreshold is the cosine-similarity floor a top hit must
	// clear before the hint is injected — keeps the prefix clean on throwaway
	// turns ("thanks!") where nothing is actually relevant.
	defaultRecallThreshold = 0.55
)

type Agent struct {
	brain         brain
	system        string
	prosopons     prosopons
	conversations conversations
	messages      messages
	queue         queue
	tools         toolRunner
	embedder      embedder
	memories      memories
	recallK       int
	recallFloor   float64
}

func New(
	b brain,
	systemPrompt string,
	prosopons prosopons,
	conversations conversations,
	messages messages,
	q queue,
	tools toolRunner,
	embedder embedder,
	memories memories,
) *Agent {
	return &Agent{
		brain:         b,
		system:        systemPrompt,
		prosopons:     prosopons,
		conversations: conversations,
		messages:      messages,
		queue:         q,
		tools:         tools,
		embedder:      embedder,
		memories:      memories,
		recallK:       defaultRecallK,
		recallFloor:   defaultRecallThreshold,
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
	hist := make([]llm.Message, 0, len(stored)+2)
	for _, m := range stored {
		role := "user"
		if m.ProsoponId == celineProsoponId {
			role = "assistant"
		}
		hist = append(hist, llm.Message{Role: role, Text: m.Content})
	}

	// Tier-2 recall (§12.5): embed the query, search this client's memories,
	// and inject a hint only when the top hit clears the similarity floor.
	// Sits below the cached prefix, so on a miss the prefix stays clean.
	if hint := a.recallHint(ctx, p.Id, userText); hint != "" {
		hist = append(hist, llm.Message{Role: "user", Text: hint})
	}

	hist = append(hist, llm.Message{Role: "user", Text: userText})

	// Tier-1 recall (§12.5): the client's curated profile, always in the
	// (cached, per-client-stable) system prefix.
	system := a.system
	if p.Persona != nil && strings.TrimSpace(*p.Persona) != "" {
		system += "\n\n# What you know about this person\n" + strings.TrimSpace(*p.Persona)
	}

	userMsg := &mneme.Message{ConversationId: convID, ProsoponId: p.Id, Content: userText}
	if err := a.messages.Create(ctx, userMsg); err != nil {
		return convID, err
	}
	a.enqueue(ctx, userMsg.Id)

	// Stamp the caller's identity so owner-scoped tools (recall, §12.5 tier 3)
	// can filter their search to this client's memories.
	ctx = ergon.WithOwner(ctx, p.Id)

	// seq tracks the global bubble index across all tool-loop iterations.
	var seq int32
	// allBubbles accumulates every bubble across iterations for DB persistence.
	var allBubbles []string

	// emit forwards each bubble to the sink the moment its boundary arrives
	// in the token stream — the client sees it while the turn is still generating.
	emit := func(bubble string) {
		if err := sink.Bubble(seq, bubble); err != nil {
			log.Printf("agent: bubble %d: %v", seq, err)
		}
		seq++
	}

	for {
		turn, err := a.brain.Chat(ctx, system, hist, a.tools.Defs(), emit)
		if err != nil {
			return convID, err
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

			// On a tool failure, Claude gets the real error (so it can reason
			// about / report the failure honestly), but the client only sees a
			// generic notice — provider/internal details are logged, not streamed.
			clientOutput := output
			if isErr {
				output = execErr.Error()
				log.Printf("agent: tool %q failed: %v", use.Name, execErr)
				clientOutput = "tool unavailable"
			}
			_ = sink.ToolResult(use.Id, clientOutput, isErr)

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

// recallHint runs the §12.5 tier-2 read path: embed the query, search this
// client's memories, and return a hint block only if the top hit clears the
// similarity floor. Best-effort — any failure (no embedder, Ollama down, empty
// store) logs and yields "", so recall never breaks a turn.
func (a *Agent) recallHint(ctx context.Context, ownerProsoponId int64, query string) string {
	if a.embedder == nil || a.memories == nil || strings.TrimSpace(query) == "" {
		return ""
	}

	vec, err := a.embedder.Embed(ctx, query)
	if err != nil {
		log.Printf("agent: recall embed: %v", err)
		return ""
	}

	hits, err := a.memories.Search(ctx, ownerProsoponId, vec, a.recallK)
	if err != nil {
		log.Printf("agent: recall search: %v", err)
		return ""
	}
	if len(hits) == 0 || hits[0].Similarity < a.recallFloor {
		return ""
	}

	var b strings.Builder
	b.WriteString("(Relevant memories from earlier — use only if helpful, don't mention this note.)\n")
	for _, h := range hits {
		if h.Similarity < a.recallFloor {
			continue
		}
		who := "them"
		if h.ProsoponId == celineProsoponId {
			who = "you"
		}
		b.WriteString("- (")
		b.WriteString(who)
		b.WriteString(") ")
		b.WriteString(strings.TrimSpace(h.Content))
		b.WriteByte('\n')
	}
	return strings.TrimRight(b.String(), "\n")
}

func (a *Agent) enqueue(ctx context.Context, msgID int64) {
	if err := a.queue.Enqueue(ctx, arche.GrapheQueue, msgID); err != nil {
		log.Printf("agent: enqueue %d: %v", msgID, err)
	}
}
