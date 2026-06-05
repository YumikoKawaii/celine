package hermes

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const tokenTTL = 30 * 24 * time.Hour

type Issuer struct {
	secret []byte
}

func NewIssuer(secret string) *Issuer {
	return &Issuer{secret: []byte(secret)}
}

// Issue signs a new JWT for the given sub, valid for 30 days.
func (i *Issuer) Issue(sub string) (string, error) {
	claims := jwt.RegisteredClaims{
		Subject:   sub,
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(tokenTTL)),
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return tok.SignedString(i.secret)
}

type Verifier struct {
	secret []byte
}

func NewVerifier(secret string) *Verifier {
	return &Verifier{secret: []byte(secret)}
}

// Verify parses and validates the token, returning the sub claim.
func (v *Verifier) Verify(tokenStr string) (string, error) {
	tok, err := jwt.ParseWithClaims(tokenStr, &jwt.RegisteredClaims{},
		func(t *jwt.Token) (any, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
			}
			return v.secret, nil
		},
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return "", err
	}
	claims, ok := tok.Claims.(*jwt.RegisteredClaims)
	if !ok || claims.Subject == "" {
		return "", errors.New("invalid claims")
	}
	return claims.Subject, nil
}
