package rpc

import (
	"context"
	"errors"

	"connectrpc.com/connect"

	celinev1 "github.com/YumikoKawaii/celine/basis/gen/celine/v1"
	"github.com/YumikoKawaii/celine/basis/gen/celine/v1/celinev1connect"
	"github.com/YumikoKawaii/celine/basis/internal/hermes"
	"github.com/YumikoKawaii/celine/basis/internal/mneme"
)

// clientStore is the subset of mneme.ClientStore this handler needs.
type clientStore interface {
	Upsert(ctx context.Context, c mneme.Client) error
	Get(ctx context.Context, sub string) (mneme.Client, error)
}

// google and issuer are kept as concrete types because they are optionally nil
// in dev mode (no GOOGLE_CLIENT_ID set). Assigning a nil concrete pointer to an
// interface produces a non-nil interface value that panics on method call — the
// nil concrete pointer is the safe, checkable form here.
type HermesService struct {
	celinev1connect.UnimplementedHermesHandler
	google  *hermes.GoogleAuth
	issuer  *hermes.Issuer
	clients clientStore
}

func NewHermesService(g *hermes.GoogleAuth, issuer *hermes.Issuer, clients clientStore) *HermesService {
	return &HermesService{google: g, issuer: issuer, clients: clients}
}

func (s *HermesService) Eisodos(
	_ context.Context,
	_ *connect.Request[celinev1.EisodosRequest],
) (*connect.Response[celinev1.EisodosResponse], error) {
	if s.google == nil {
		return nil, connect.NewError(connect.CodeUnimplemented, errors.New("google auth not configured"))
	}
	state := hermes.NewState()
	url := s.google.AuthURL("", state)
	return connect.NewResponse(&celinev1.EisodosResponse{Url: url, State: state}), nil
}

func (s *HermesService) Metabole(
	ctx context.Context,
	req *connect.Request[celinev1.MetaboleRequest],
) (*connect.Response[celinev1.MetaboleResponse], error) {
	if s.google == nil || s.issuer == nil {
		return nil, connect.NewError(connect.CodeUnimplemented, errors.New("google auth not configured"))
	}

	claims, err := s.google.Exchange(ctx, req.Msg.GetCode(), req.Msg.GetRedirectUri())
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	if err := s.clients.Upsert(ctx, mneme.Client{
		Sub:         claims.Sub,
		Email:       claims.Email,
		DisplayName: claims.Name,
		AvatarURL:   claims.Picture,
	}); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	token, err := s.issuer.Issue(claims.Sub)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&celinev1.MetaboleResponse{
		Token: token,
		User: &celinev1.User{
			Sub:         claims.Sub,
			Email:       claims.Email,
			DisplayName: claims.Name,
			AvatarUrl:   claims.Picture,
		},
	}), nil
}

func (s *HermesService) Gnorizo(
	ctx context.Context,
	_ *connect.Request[celinev1.GnorizoRequest],
) (*connect.Response[celinev1.GnorizoResponse], error) {
	sub, ok := hermes.SubFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("not authenticated"))
	}

	client, err := s.clients.Get(ctx, sub)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&celinev1.GnorizoResponse{
		User: &celinev1.User{
			Sub:         client.Sub,
			Email:       client.Email,
			DisplayName: client.DisplayName,
			AvatarUrl:   client.AvatarURL,
		},
	}), nil
}

func (s *HermesService) Exodos(
	_ context.Context,
	_ *connect.Request[celinev1.ExodosRequest],
) (*connect.Response[celinev1.ExodosResponse], error) {
	return connect.NewResponse(&celinev1.ExodosResponse{}), nil
}
