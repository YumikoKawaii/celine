package rpc

import (
	"context"
	"errors"
	"log"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	celinev1 "github.com/YumikoKawaii/celine/basis/gen/celine/v1"
	"github.com/YumikoKawaii/celine/basis/gen/celine/v1/celinev1connect"
	"github.com/YumikoKawaii/celine/basis/internal/hermes"
	"github.com/YumikoKawaii/celine/basis/internal/mneme"
)

type Celine struct {
	celinev1connect.UnimplementedCelineHandler
	agent    chatAgent
	msgs     msgReader
	convs    convResolver
	registry *registry
	debounce time.Duration
}

func NewCeline(a chatAgent, msgs msgReader, convs convResolver, debounce time.Duration) *Celine {
	return &Celine{
		agent: a,
		msgs:  msgs,
		convs: convs,
		registry: &registry{
			sigao: make(map[string]chan struct{}),
			pempo: make(map[string]chan string),
		},
		debounce: debounce,
	}
}

func (s *Celine) Parousia(
	ctx context.Context,
	_ *connect.Request[celinev1.ParousiaRequest],
	stream *connect.ServerStream[celinev1.ParousiaEvent],
) error {
	sub, ok := hermes.SubFromContext(ctx)
	if !ok {
		return connect.NewError(connect.CodeUnauthenticated, errors.New("not authenticated"))
	}

	s.registry.Register(sub)
	defer s.registry.Unregister(sub)

	sigao, _ := s.registry.Sigao(sub)
	pempo, _ := s.registry.Pempo(sub)
	ms := messages{}

	sink := &streamSink{stream: stream}
	ticker := time.NewTicker(s.debounce)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case v := <-pempo:
			ms.Enqueue(v)
			ticker.Reset(s.debounce)
		case <-ticker.C:
			select {
			case sigao <- struct{}{}:
			default: // signal already pending, coalesce
			}
		case <-sigao:
			packet := ms.Flush()
			if packet == "" {
				ticker.Reset(s.debounce)
				continue
			}
			if err := sink.typing(); err != nil {
				return err
			}
			id, err := s.agent.Chat(ctx, sub, packet, sink)
			if err != nil {
				// The turn failed (LLM/embedder/DB hiccup). Log the real cause
				// server-side, hand the client a safe generic message, and keep
				// the session open — one bad turn must not drop the stream.
				if ctx.Err() != nil {
					return nil
				}
				log.Printf("parousia: chat turn for %s failed: %v", sub, err)
				if sendErr := sink.fail(); sendErr != nil {
					return sendErr
				}
				ticker.Reset(s.debounce)
				continue
			}
			if err := sink.done(id); err != nil {
				return err
			}
			ticker.Reset(s.debounce)
		}
	}
}

func (s *Celine) Pempo(
	ctx context.Context,
	req *connect.Request[celinev1.PempoRequest],
) (*connect.Response[celinev1.PempoResponse], error) {
	sub, ok := hermes.SubFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("not authenticated"))
	}

	conn, ok := s.registry.Pempo(sub)
	if !ok {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("no active session — open Parousia first"))
	}
	select {
	case conn <- req.Msg.GetText():
		return connect.NewResponse(&celinev1.PempoResponse{}), nil
	case <-ctx.Done():
		return nil, connect.NewError(connect.CodeCanceled, ctx.Err())
	}
}

func (s *Celine) Sigao(
	ctx context.Context,
	_ *connect.Request[celinev1.SigaoRequest],
) (*connect.Response[celinev1.SigaoResponse], error) {
	sub, ok := hermes.SubFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("not authenticated"))
	}

	conn, ok := s.registry.Sigao(sub)
	if !ok {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("no active session — open Parousia first"))
	}

	select {
	case conn <- struct{}{}:
	default: // a quiet signal is already pending — coalesce
	}
	return connect.NewResponse(&celinev1.SigaoResponse{}), nil
}

// Anamnesis returns the messages in the user's conversation, oldest first.
// The conversation ID comes from the JWT claim — no DB lookup or ownership check needed.
func (s *Celine) Anamnesis(
	ctx context.Context,
	_ *connect.Request[celinev1.AnamnesisRequest],
) (*connect.Response[celinev1.AnamnesisResponse], error) {
	convID, ok := hermes.ConversationIDFromContext(ctx)
	if !ok {
		// No conversation claim (dev-anon path): resolve it from the prosopon.
		pid, hasPid := hermes.ProsoponIdFromContext(ctx)
		if !hasPid {
			return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("not authenticated"))
		}
		conv, err := s.convs.GetOrCreate(ctx, mneme.KataProsopon{ProsoponId: pid})
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		convID = conv.Id
	}

	msgs, err := s.msgs.List(ctx, anamnesisMessages{convID: convID}, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	out := make([]*celinev1.ChatMessage, len(msgs))
	for i, m := range msgs {
		out[i] = &celinev1.ChatMessage{
			Id:         m.Id,
			ProsoponId: m.ProsoponId,
			Text:       m.Content,
			CreatedAt:  timestamppb.New(m.CreatedAt),
		}
	}
	return connect.NewResponse(&celinev1.AnamnesisResponse{Messages: out}), nil
}

// streamSink writes agent events directly onto the Parousia stream. The agent
// turn runs on the Parousia goroutine, so there is exactly one writer and no
// buffering — a slow client backpressures the turn instead of dropping events.
type streamSink struct {
	stream *connect.ServerStream[celinev1.ParousiaEvent]
}

func (s *streamSink) typing() error {
	return s.stream.Send(&celinev1.ParousiaEvent{
		Event: &celinev1.ParousiaEvent_Typing{Typing: &celinev1.Typing{}},
	})
}

func (s *streamSink) done(conversationId int64) error {
	return s.stream.Send(&celinev1.ParousiaEvent{
		Event: &celinev1.ParousiaEvent_Done{Done: &celinev1.Done{ConversationId: conversationId}},
	})
}

// clientErrorMessage is the only error text a client ever sees for a failed
// turn — raw errors from Claude / Ollama / Postgres are logged server-side, not
// forwarded, so internal details and provider specifics never leak.
const clientErrorMessage = "Something went wrong on my end. Please try again in a moment."

func (s *streamSink) fail() error {
	return s.stream.Send(&celinev1.ParousiaEvent{
		Event: &celinev1.ParousiaEvent_Error{Error: clientErrorMessage},
	})
}

func (s *streamSink) Bubble(seq int32, text string) error {
	return s.stream.Send(&celinev1.ParousiaEvent{
		Event: &celinev1.ParousiaEvent_Message{Message: &celinev1.Message{Seq: seq, Text: text}},
	})
}

func (s *streamSink) ToolCall(id, name, inputJSON string) error {
	return s.stream.Send(&celinev1.ParousiaEvent{
		Event: &celinev1.ParousiaEvent_ToolCall{ToolCall: &celinev1.ToolCall{
			Id: id, Name: name, InputJson: inputJSON,
		}},
	})
}

func (s *streamSink) ToolResult(id, output string, isError bool) error {
	return s.stream.Send(&celinev1.ParousiaEvent{
		Event: &celinev1.ParousiaEvent_ToolResult{ToolResult: &celinev1.ToolResult{
			Id: id, Output: output, IsError: isError,
		}},
	})
}
