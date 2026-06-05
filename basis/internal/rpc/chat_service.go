package rpc

import (
	"context"
	"strings"
	"time"

	"connectrpc.com/connect"

	celinev1 "github.com/YumikoKawaii/celine/basis/gen/celine/v1"
	"github.com/YumikoKawaii/celine/basis/gen/celine/v1/celinev1connect"
)

// CelineService implements the Connect CelineServiceHandler.
//
// Step 1 (plumbing, architecture.md §8): no Claude, no DB. Chat streams a
// hardcoded, paced, multi-bubble reply to prove the typed stream end to end.
// The pacing here is the §14 backend pacer in miniature: typing beat → bubble.
type CelineService struct {
	celinev1connect.UnimplementedCelineServiceHandler
}

func NewCelineService() *CelineService { return &CelineService{} }

// §14.3 typing-delay shape: clamp(BASE + PER_CHAR·len, _, MAX).
const (
	typingBaseMs  = 400
	typingPerChar = 25
	typingMaxMs   = 2500
	interBubbleMs = 500
)

func typingDelayMs(s string) int32 {
	d := typingBaseMs + typingPerChar*len(s)
	if d > typingMaxMs {
		d = typingMaxMs
	}
	return int32(d)
}

func (s *CelineService) Chat(
	ctx context.Context,
	req *connect.Request[celinev1.ChatRequest],
	stream *connect.ServerStream[celinev1.ChatEvent],
) error {
	in := strings.TrimSpace(req.Msg.GetText())
	if in == "" {
		in = "..."
	}

	// A segmented reply (one thought per bubble, §14). Echoes the input so we
	// can see the pipe carrying real data.
	reply := []string{
		"yahalo — Celine here.",
		"the typed pipe works: proto → Connect stream → your browser.",
		"you said: \"" + in + "\"",
		"no brain yet, just plumbing. but she's listening.",
	}

	for i, bubble := range reply {
		delay := typingDelayMs(bubble)
		if err := stream.Send(&celinev1.ChatEvent{
			Event: &celinev1.ChatEvent_Typing{Typing: &celinev1.Typing{MsHint: delay}},
		}); err != nil {
			return err
		}
		if err := sleep(ctx, time.Duration(delay)*time.Millisecond); err != nil {
			return err
		}

		if err := stream.Send(&celinev1.ChatEvent{
			Event: &celinev1.ChatEvent_Message{
				Message: &celinev1.Message{Seq: int32(i), Text: bubble},
			},
		}); err != nil {
			return err
		}
		if err := sleep(ctx, interBubbleMs*time.Millisecond); err != nil {
			return err
		}
	}

	convID := req.Msg.GetConversationId()
	if convID == "" {
		convID = "conv-dev-1"
	}
	return stream.Send(&celinev1.ChatEvent{
		Event: &celinev1.ChatEvent_Done{Done: &celinev1.Done{ConversationId: convID}},
	})
}

// sleep waits for d unless the request context is cancelled first.
func sleep(ctx context.Context, d time.Duration) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(d):
		return nil
	}
}
