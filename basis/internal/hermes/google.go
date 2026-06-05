package hermes

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type GoogleClaims struct {
	Sub     string
	Email   string
	Name    string
	Picture string
}

type GoogleAuth struct {
	cfg *oauth2.Config
}

func NewGoogleAuth(clientID, clientSecret string) *GoogleAuth {
	return &GoogleAuth{
		cfg: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			Scopes:       []string{"openid", "email", "profile"},
			Endpoint:     google.Endpoint,
		},
	}
}

// AuthURL returns the Google OAuth consent URL for the given state token.
func (g *GoogleAuth) AuthURL(redirectURI, state string) string {
	cfg := g.cfgWithRedirect(redirectURI)
	return cfg.AuthCodeURL(state, oauth2.AccessTypeOnline)
}

// Exchange trades an OAuth code for Google claims. The id_token is decoded
// without JWKS signature verification because it was received directly from
// Google's token endpoint over a server-side HTTPS exchange using our
// client_secret — the transport already guarantees authenticity.
// TODO: add JWKS verification for defence in depth.
func (g *GoogleAuth) Exchange(ctx context.Context, code, redirectURI string) (GoogleClaims, error) {
	cfg := g.cfgWithRedirect(redirectURI)
	tok, err := cfg.Exchange(ctx, code)
	if err != nil {
		return GoogleClaims{}, fmt.Errorf("google exchange: %w", err)
	}

	idToken, ok := tok.Extra("id_token").(string)
	if !ok || idToken == "" {
		return GoogleClaims{}, fmt.Errorf("google exchange: no id_token in response")
	}

	claims, err := decodeIDToken(idToken)
	if err != nil {
		return GoogleClaims{}, fmt.Errorf("google exchange: %w", err)
	}
	return claims, nil
}

func (g *GoogleAuth) cfgWithRedirect(redirectURI string) *oauth2.Config {
	cp := *g.cfg
	cp.RedirectURL = redirectURI
	return &cp
}

// decodeIDToken decodes the payload of a JWT without verifying the signature.
func decodeIDToken(token string) (GoogleClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return GoogleClaims{}, fmt.Errorf("malformed id_token")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return GoogleClaims{}, fmt.Errorf("decode id_token payload: %w", err)
	}
	var raw struct {
		Sub     string `json:"sub"`
		Email   string `json:"email"`
		Name    string `json:"name"`
		Picture string `json:"picture"`
	}
	if err := json.Unmarshal(payload, &raw); err != nil {
		return GoogleClaims{}, fmt.Errorf("unmarshal id_token: %w", err)
	}
	return GoogleClaims{
		Sub:     raw.Sub,
		Email:   raw.Email,
		Name:    raw.Name,
		Picture: raw.Picture,
	}, nil
}

// NewState generates a random CSRF state token for the OAuth flow.
func NewState() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

