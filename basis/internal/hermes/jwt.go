package hermes

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const DefaultTokenTTL = 72 * time.Hour

// Claims is the payload of every server-issued JWT.
// pid and cid are included so handlers can skip DB lookups on every RPC.
type Claims struct {
	jwt.RegisteredClaims
	ProsoponId     int64 `json:"pid"`
	ConversationId int64 `json:"cid"`
}

type Issuer struct {
	secret []byte
	ttl    time.Duration
}

func NewIssuer(secret string, ttl time.Duration) *Issuer {
	return &Issuer{secret: []byte(secret), ttl: ttl}
}

// Issue signs a new JWT for the given sub, prosopon ID, and conversation ID,
// valid for the configured TTL.
func (i *Issuer) Issue(sub string, prosoponId, convId int64) (string, error) {
	now := time.Now()
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   sub,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(i.ttl)),
		},
		ProsoponId:     prosoponId,
		ConversationId: convId,
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

// Verify parses and validates the token, returning the full claims.
func (v *Verifier) Verify(tokenStr string) (Claims, error) {
	var claims Claims
	tok, err := jwt.ParseWithClaims(tokenStr, &claims,
		func(t *jwt.Token) (any, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
			}
			return v.secret, nil
		},
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return Claims{}, err
	}
	if !tok.Valid || claims.Subject == "" || claims.ProsoponId == 0 {
		return Claims{}, errors.New("invalid claims")
	}
	return claims, nil
}
