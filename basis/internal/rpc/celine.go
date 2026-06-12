package rpc

import (
	"context"
	"errors"
	"strconv"
	"time"

	"connectrpc.com/connect"

	celinev1 "github.com/YumikoKawaii/celine/basis/gen/celine/v1"
	"github.com/YumikoKawaii/celine/basis/gen/celine/v1/celinev1connect"
	"github.com/YumikoKawaii/celine/basis/internal/agent"
	"github.com/YumikoKawaii/celine/basis/internal/hermes"
)

type chatAgent interface {
	Chat(ctx context.Context, ownerSub string, userText string, sink agent.EventSink) (int64, error)
}

type Celine struct {
	celinev1connect.UnimplementedCelineHandler
	agent    chatAgent
	sessions *sessionStore
	debounce time.Duration
}

func NewCeline(a chatAgent, debounce time.Duration) *Celine {
	return &Celine{agent: a, sessions: newSessionStore(), debounce: debounce}
}

func (s *Celine) Parousia(
	ctx context.Context,
	req *connect.Request[celinev1.ParousiaRequest],
	stream *connect.ServerStream[celinev1.ParousiaEvent],
) error {
	sub, ok := hermes.SubFromContext(ctx)
	if !ok {
		return connect.NewError(connect.CodeUnauthenticated, errors.New("not authenticated"))
	}

	sess := s.sessions.register(sub, ctx, s.debounce, s.runFlush)
	defer s.sessions.unregister(sub)

	for {
		select {
		case <-ctx.Done():
			return nil
		case ev := <-sess.ch:
			if err := stream.Send(ev); err != nil {
				return err
			}
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

	sess, ok := s.sessions.get(sub)
	if !ok {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("no active session — open Parousia first"))
	}

	sess.pempo(req.Msg.GetText())
	return connect.NewResponse(&celinev1.PempoResponse{}), nil
}

func (s *Celine) runFlush(sub string) {
	sess, ok := s.sessions.get(sub)
	if !ok {
		return
	}

	combined := sess.drainInbox()
	if combined == "" {
		sess.clearBusy()
		return
	}

	sink := newChanSink(sess.ch)
	id, err := s.agent.Chat(sess.ctx, sub, combined, sink)
	if err != nil {
		sink.send(&celinev1.ParousiaEvent{
			Event: &celinev1.ParousiaEvent_Error{Error: err.Error()},
		})
	} else {
		sink.send(&celinev1.ParousiaEvent{
			Event: &celinev1.ParousiaEvent_Done{Done: &celinev1.Done{
				ConversationId: strconv.FormatInt(id, 10),
			}},
		})
	}

	sess.clearBusy()
}

type chanSink struct {
	ch chan<- *celinev1.ParousiaEvent
}

func newChanSink(ch chan *celinev1.ParousiaEvent) *chanSink {
	return &chanSink{ch: ch}
}

func (s *chanSink) send(ev *celinev1.ParousiaEvent) {
	select {
	case s.ch <- ev:
	default:
	}
}

func (s *chanSink) Typing(msHint int32) error {
	s.send(&celinev1.ParousiaEvent{
		Event: &celinev1.ParousiaEvent_Typing{Typing: &celinev1.Typing{MsHint: msHint}},
	})
	return nil
}

func (s *chanSink) Bubble(seq int32, text string) error {
	s.send(&celinev1.ParousiaEvent{
		Event: &celinev1.ParousiaEvent_Message{Message: &celinev1.Message{Seq: seq, Text: text}},
	})
	return nil
}

func (s *chanSink) ToolCall(id, name, inputJSON string) error {
	s.send(&celinev1.ParousiaEvent{
		Event: &celinev1.ParousiaEvent_ToolCall{ToolCall: &celinev1.ToolCall{
			Id: id, Name: name, InputJson: inputJSON,
		}},
	})
	return nil
}

func (s *chanSink) ToolResult(id, output string, isError bool) error {
	s.send(&celinev1.ParousiaEvent{
		Event: &celinev1.ParousiaEvent_ToolResult{ToolResult: &celinev1.ToolResult{
			Id: id, Output: output, IsError: isError,
		}},
	})
	return nil
}
