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

type HermesService struct {
	celinev1connect.UnimplementedHermesHandler
	google  *hermes.GoogleAuth
	issuer  *hermes.Issuer
	clients *mneme.ClientStore
}

func NewHermesService(g *hermes.GoogleAuth, issuer *hermes.Issuer, clients *mneme.ClientStore) *HermesService {
	return &HermesService{google: g, issuer: issuer, clients: clients}
}

func (s *HermesService) Eisodos(
	_ context.Context,
	req *connect.Request[celinev1.EisodosRequest],
) (*connect.Response[celinev1.EisodosResponse], error) {
	state := hermes.NewState()
	// redirect_uri is provided by the client on ExchangeGoogleCode; we don't
	// need it here since AuthURL is called with an empty redirect and the
	// client supplies the real one in Metabole.
	url := s.google.AuthURL("", state)
	return connect.NewResponse(&celinev1.EisodosResponse{Url: url, State: state}), nil
}

func (s *HermesService) Metabole(
	ctx context.Context,
	req *connect.Request[celinev1.MetaboleRequest],
) (*connect.Response[celinev1.MetaboleResponse], error) {
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
	req *connect.Request[celinev1.GnorizoRequest],
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
	// JWT is stateless — logout is handled client-side by discarding the token.
	return connect.NewResponse(&celinev1.ExodosResponse{}), nil
}
