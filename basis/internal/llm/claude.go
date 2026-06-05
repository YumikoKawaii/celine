package llm

import (
	"context"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

type Message struct {
	Role string
	Text string
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

func (c *Client) StreamChat(ctx context.Context, system string, history []Message, deltas chan<- string) (string, error) {
	defer close(deltas)

	msgs := make([]anthropic.MessageParam, 0, len(history))
	for _, m := range history {
		block := anthropic.NewTextBlock(m.Text)
		if m.Role == "assistant" {
			msgs = append(msgs, anthropic.NewAssistantMessage(block))
		} else {
			msgs = append(msgs, anthropic.NewUserMessage(block))
		}
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

	stream := c.api.Messages.NewStreaming(ctx, params)
	var buf strings.Builder
	for stream.Next() {
		ev := stream.Current()
		if ev.Type == "content_block_delta" && ev.Delta.Text != "" {
			buf.WriteString(ev.Delta.Text)
			select {
			case deltas <- ev.Delta.Text:
			case <-ctx.Done():
				return buf.String(), ctx.Err()
			}
		}
	}
	if err := stream.Err(); err != nil {
		return buf.String(), err
	}
	return buf.String(), nil
}
