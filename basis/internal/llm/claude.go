package llm

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// Message is a conversation turn. Tool fields are only populated for
// in-turn tool-use rounds — they are never stored in Postgres.
type Message struct {
	Role        string       // "user" | "assistant"
	Text        string       // plain text content
	ToolUses    []ToolUse    // assistant: tool_use blocks
	ToolResults []ToolResult // user: tool_result blocks
}

// ToolDef is a tool declaration passed to Claude in MessageNewParams.
type ToolDef struct {
	Name        string
	Description string
	Schema      map[string]any // full JSON schema: {type, properties, required, ...}
}

// ToolUse is one tool_use block from an assistant response.
type ToolUse struct {
	Id    string
	Name  string
	Input json.RawMessage
}

// ToolResult is one tool_result block to feed back to Claude.
type ToolResult struct {
	Id      string
	Output  string
	IsError bool
}

// Turn is the completed result of one Chat call. Bubbles contains the assistant
// text split on \n\n boundaries (code-fence-aware), ready to send to the client.
// Uses is non-empty when stop_reason = tool_use.
type Turn struct {
	Bubbles []string
	Uses    []ToolUse
}

type Client struct {
	api       anthropic.Client
	model     anthropic.Model
	maxTokens int64
}

const DefaultMaxTokens int64 = 8192

func New(apiKey, model string, maxTokens int64) *Client {
	if model == "" {
		model = anthropic.ModelClaudeSonnet4_6
	}
	if maxTokens <= 0 {
		maxTokens = DefaultMaxTokens
	}
	return &Client{
		api:       anthropic.NewClient(option.WithAPIKey(apiKey)),
		model:     model,
		maxTokens: maxTokens,
	}
}

// Chat streams an assistant response and returns the completed turn. Text is
// split into bubbles at \n\n boundaries (code-fence-aware) as it streams;
// each completed bubble is handed to emit the moment its boundary arrives,
// so the caller can deliver it while the rest of the turn is still generating.
// The returned Turn carries the full bubble list for persistence.
func (c *Client) Chat(
	ctx context.Context,
	system string,
	history []Message,
	tools []ToolDef,
	emit func(bubble string),
) (Turn, error) {
	if emit == nil {
		emit = func(string) {}
	}
	msgs := make([]anthropic.MessageParam, 0, len(history))
	for _, m := range history {
		msgs = append(msgs, toParam(m))
	}

	params := anthropic.MessageNewParams{
		Model:     c.model,
		MaxTokens: c.maxTokens,
		Messages:  msgs,
	}
	if system != "" {
		params.System = []anthropic.TextBlockParam{{
			Text:         system,
			CacheControl: anthropic.NewCacheControlEphemeralParam(),
		}}
	}
	if len(tools) > 0 {
		params.Tools = make([]anthropic.ToolUnionParam, len(tools))
		for i, t := range tools {
			var required []string
			if req, ok := t.Schema["required"].([]string); ok {
				required = req
			}
			params.Tools[i] = anthropic.ToolUnionParam{
				OfTool: &anthropic.ToolParam{
					Name:        t.Name,
					Description: anthropic.String(t.Description),
					InputSchema: anthropic.ToolInputSchemaParam{
						Properties: t.Schema["properties"],
						Required:   required,
					},
				},
			}
		}
	}

	stream := c.api.Messages.NewStreaming(ctx, params)
	var acc anthropic.Message
	var buf strings.Builder
	var bubbles []string

	for stream.Next() {
		ev := stream.Current()
		_ = acc.Accumulate(ev)
		if ev.Type != "content_block_delta" || ev.Delta.Text == "" {
			continue
		}
		buf.WriteString(ev.Delta.Text)
		// flush all complete bubbles from the buffer
		for {
			s := buf.String()
			idx := bubbleBoundary(s)
			if idx < 0 {
				break
			}
			if b := strings.TrimSpace(s[:idx]); b != "" {
				bubbles = append(bubbles, b)
				emit(b)
			}
			buf.Reset()
			buf.WriteString(s[idx+2:])
		}
	}
	if err := stream.Err(); err != nil {
		return Turn{}, err
	}
	// flush whatever remains after the stream closes
	if b := strings.TrimSpace(buf.String()); b != "" {
		bubbles = append(bubbles, b)
		emit(b)
	}

	turn := Turn{Bubbles: bubbles}
	if acc.StopReason == anthropic.StopReasonToolUse {
		for _, block := range acc.Content {
			if block.Type == "tool_use" {
				turn.Uses = append(turn.Uses, ToolUse{
					Id:    block.ID,
					Name:  block.Name,
					Input: block.Input,
				})
			}
		}
	}
	return turn, nil
}

// bubbleBoundary returns the index of the first \n\n in s that is not inside
// a code fence. Returns -1 if no such boundary exists.
func bubbleBoundary(s string) int {
	inFence := false
	for i := 0; i+1 < len(s); i++ {
		if strings.HasPrefix(s[i:], "```") {
			inFence = !inFence
			i += 2
			continue
		}
		if !inFence && s[i] == '\n' && s[i+1] == '\n' {
			return i
		}
	}
	return -1
}

// toParam converts a llm.Message to the anthropic SDK's MessageParam.
func toParam(m Message) anthropic.MessageParam {
	// User message carrying tool results.
	if len(m.ToolResults) > 0 {
		blocks := make([]anthropic.ContentBlockParamUnion, len(m.ToolResults))
		for i, r := range m.ToolResults {
			blocks[i] = anthropic.NewToolResultBlock(r.Id, r.Output, r.IsError)
		}
		return anthropic.NewUserMessage(blocks...)
	}
	// Assistant message carrying tool_use blocks (may also include text).
	if len(m.ToolUses) > 0 {
		blocks := make([]anthropic.ContentBlockParamUnion, 0, len(m.ToolUses)+1)
		if m.Text != "" {
			blocks = append(blocks, anthropic.NewTextBlock(m.Text))
		}
		for _, u := range m.ToolUses {
			var input any
			_ = json.Unmarshal(u.Input, &input)
			blocks = append(blocks, anthropic.NewToolUseBlock(u.Id, input, u.Name))
		}
		return anthropic.NewAssistantMessage(blocks...)
	}
	// Plain text message.
	block := anthropic.NewTextBlock(m.Text)
	if m.Role == "assistant" {
		return anthropic.NewAssistantMessage(block)
	}
	return anthropic.NewUserMessage(block)
}
