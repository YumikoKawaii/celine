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
	ID    string
	Name  string
	Input json.RawMessage
}

// ToolResult is one tool_result block to feed back to Claude.
type ToolResult struct {
	ID      string
	Output  string
	IsError bool
}

// Turn is the completed result of one StreamChat call.
type Turn struct {
	Text string    // accumulated assistant text
	Uses []ToolUse // non-empty when stop_reason = tool_use
}

type Client struct {
	api       anthropic.Client
	model     anthropic.Model
	maxTokens int64
}

func New(apiKey, model string) *Client {
	m := anthropic.Model(model)
	if model == "" {
		m = anthropic.ModelClaudeOpus4_8
	}
	return &Client{
		api:       anthropic.NewClient(option.WithAPIKey(apiKey)),
		model:     m,
		maxTokens: 2048,
	}
}

// StreamChat streams an assistant response. Text deltas are sent on deltas
// (closed when streaming ends). The returned Turn contains the full text and
// any tool_use blocks if stop_reason = tool_use.
func (c *Client) StreamChat(
	ctx context.Context,
	system string,
	history []Message,
	tools []ToolDef,
	deltas chan<- string,
) (Turn, error) {
	defer close(deltas)

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
	for stream.Next() {
		ev := stream.Current()
		_ = acc.Accumulate(ev)
		if ev.Type == "content_block_delta" && ev.Delta.Text != "" {
			select {
			case deltas <- ev.Delta.Text:
			case <-ctx.Done():
				return Turn{Text: textFrom(acc)}, ctx.Err()
			}
		}
	}
	if err := stream.Err(); err != nil {
		return Turn{}, err
	}

	turn := Turn{Text: textFrom(acc)}
	if acc.StopReason == anthropic.StopReasonToolUse {
		for _, block := range acc.Content {
			if block.Type == "tool_use" {
				turn.Uses = append(turn.Uses, ToolUse{
					ID:    block.ID,
					Name:  block.Name,
					Input: block.Input,
				})
			}
		}
	}
	return turn, nil
}

// toParam converts a llm.Message to the anthropic SDK's MessageParam.
func toParam(m Message) anthropic.MessageParam {
	// User message carrying tool results.
	if len(m.ToolResults) > 0 {
		blocks := make([]anthropic.ContentBlockParamUnion, len(m.ToolResults))
		for i, r := range m.ToolResults {
			blocks[i] = anthropic.NewToolResultBlock(r.ID, r.Output, r.IsError)
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
			blocks = append(blocks, anthropic.NewToolUseBlock(u.ID, input, u.Name))
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

// textFrom extracts concatenated text from all text content blocks.
func textFrom(msg anthropic.Message) string {
	var sb strings.Builder
	for _, block := range msg.Content {
		if block.Type == "text" {
			sb.WriteString(block.Text)
		}
	}
	return sb.String()
}
