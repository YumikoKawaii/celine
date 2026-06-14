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

// google and issuer are concrete types (not interfaces) so the nil guards in
// the handlers below are checkable: a nil concrete pointer stays nil, whereas a
// nil pointer boxed in an interface is non-nil and panics on method call. Auth
// is mandatory, so these are defensive — wiring always supplies both.
type Hermes struct {
	celinev1connect.UnimplementedHermesHandler
	google    *hermes.GoogleAuth
	issuer    *hermes.Issuer
	prosopons prosoponStore
	convs     convReader
	whitelist *hermes.Whitelist
}

func NewHermes(g *hermes.GoogleAuth, issuer *hermes.Issuer, prosopons prosoponStore, convs convReader, whitelist *hermes.Whitelist) *Hermes {
	return &Hermes{google: g, issuer: issuer, prosopons: prosopons, convs: convs, whitelist: whitelist}
}

func (s *Hermes) Eisodos(
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

func (s *Hermes) Metabole(
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

	// Whitelist gate: only listed emails may obtain a token (this is where the
	// verified email is first known). Unlisted accounts never reach the agent
	// loop, so they can't spend Anthropic usage.
	if s.whitelist != nil && !s.whitelist.Allowed(claims.Email) {
		return nil, connect.NewError(connect.CodePermissionDenied,
			errors.New("this account is not authorized to use Celine"))
	}

	p := &mneme.Prosopon{Sub: claims.Sub, Email: claims.Email, DisplayName: claims.Name, AvatarURL: &claims.Picture}
	if err := s.prosopons.Upsert(ctx, p); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	conv, err := s.convs.GetOrCreate(ctx, mneme.KataProsopon{ProsoponId: p.Id})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	token, err := s.issuer.Issue(claims.Sub, p.Id, conv.Id)
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

func (s *Hermes) Gnorizo(
	ctx context.Context,
	_ *connect.Request[celinev1.GnorizoRequest],
) (*connect.Response[celinev1.GnorizoResponse], error) {
	sub, ok := hermes.SubFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("not authenticated"))
	}

	p, err := s.prosopons.Get(ctx, mneme.KataSub{Sub: sub})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	avatarURL := ""
	if p.AvatarURL != nil {
		avatarURL = *p.AvatarURL
	}
	return connect.NewResponse(&celinev1.GnorizoResponse{
		User: &celinev1.User{
			Sub:         p.Sub,
			Email:       p.Email,
			DisplayName: p.DisplayName,
			AvatarUrl:   avatarURL,
		},
	}), nil
}

func (s *Hermes) Exodos(
	_ context.Context,
	_ *connect.Request[celinev1.ExodosRequest],
) (*connect.Response[celinev1.ExodosResponse], error) {
	return connect.NewResponse(&celinev1.ExodosResponse{}), nil
}
