package rpc

import (
	"context"

	"connectrpc.com/connect"

	celinev1 "github.com/YumikoKawaii/celine/basis/gen/celine/v1"
	"github.com/YumikoKawaii/celine/basis/gen/celine/v1/celinev1connect"
	"github.com/YumikoKawaii/celine/basis/internal/agent"
	"github.com/YumikoKawaii/celine/basis/internal/hermes"
)

type CelineService struct {
	celinev1connect.UnimplementedCelineHandler
	agent *agent.Agent
}

func NewCelineService(a *agent.Agent) *CelineService {
	return &CelineService{agent: a}
}

func (s *CelineService) Laleo(
	ctx context.Context,
	req *connect.Request[celinev1.LaleoRequest],
	stream *connect.ServerStream[celinev1.LaleoEvent],
) error {
	sink := &streamSink{stream: stream}

	sub, _ := hermes.SubFromContext(ctx) // set by AuthInterceptor; "anon" in dev mode
	convID, err := s.agent.Chat(ctx, sub, req.Msg.GetConversationId(), req.Msg.GetText(), sink)
	if err != nil {
		return stream.Send(&celinev1.LaleoEvent{
			Event: &celinev1.LaleoEvent_Error{Error: err.Error()},
		})
	}

	return stream.Send(&celinev1.LaleoEvent{
		Event: &celinev1.LaleoEvent_Done{Done: &celinev1.Done{ConversationId: convID}},
	})
}

type streamSink struct {
	stream *connect.ServerStream[celinev1.LaleoEvent]
}

func (s *streamSink) Typing(msHint int32) error {
	return s.stream.Send(&celinev1.LaleoEvent{
		Event: &celinev1.LaleoEvent_Typing{Typing: &celinev1.Typing{MsHint: msHint}},
	})
}

func (s *streamSink) Bubble(seq int32, text string) error {
	return s.stream.Send(&celinev1.LaleoEvent{
		Event: &celinev1.LaleoEvent_Message{Message: &celinev1.Message{Seq: seq, Text: text}},
	})
}
