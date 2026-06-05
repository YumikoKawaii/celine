package rpc

import (
	"context"

	"connectrpc.com/connect"

	celinev1 "github.com/YumikoKawaii/celine/basis/gen/celine/v1"
	"github.com/YumikoKawaii/celine/basis/gen/celine/v1/celinev1connect"
	"github.com/YumikoKawaii/celine/basis/internal/agent"
)

type CelineService struct {
	celinev1connect.UnimplementedCelineServiceHandler
	agent *agent.Agent
}

func NewCelineService(a *agent.Agent) *CelineService {
	return &CelineService{agent: a}
}

func (s *CelineService) Chat(
	ctx context.Context,
	req *connect.Request[celinev1.ChatRequest],
	stream *connect.ServerStream[celinev1.ChatEvent],
) error {
	sink := &streamSink{stream: stream}

	convID, err := s.agent.Chat(ctx, req.Msg.GetConversationId(), req.Msg.GetText(), sink)
	if err != nil {
		return stream.Send(&celinev1.ChatEvent{
			Event: &celinev1.ChatEvent_Error{Error: err.Error()},
		})
	}

	return stream.Send(&celinev1.ChatEvent{
		Event: &celinev1.ChatEvent_Done{Done: &celinev1.Done{ConversationId: convID}},
	})
}

type streamSink struct {
	stream *connect.ServerStream[celinev1.ChatEvent]
}

func (s *streamSink) Typing(msHint int32) error {
	return s.stream.Send(&celinev1.ChatEvent{
		Event: &celinev1.ChatEvent_Typing{Typing: &celinev1.Typing{MsHint: msHint}},
	})
}

func (s *streamSink) Bubble(seq int32, text string) error {
	return s.stream.Send(&celinev1.ChatEvent{
		Event: &celinev1.ChatEvent_Message{Message: &celinev1.Message{Seq: seq, Text: text}},
	})
}
