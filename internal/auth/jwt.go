package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const TokenLifetime = 8 * time.Hour

type Claims struct {
	Email string `json:"email"`
	Name  string `json:"name"`
	jwt.RegisteredClaims
}

func Mint(signingKey, issuer, email, name string) (string, error) {
	now := time.Now()
	claims := Claims{
		Email: email,
		Name:  name,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    issuer,
			Subject:   email,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(TokenLifetime)),
			NotBefore: jwt.NewNumericDate(now),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return tok.SignedString([]byte(signingKey))
}

func Verify(token, signingKey string) (*Claims, error) {
	parsed, err := jwt.ParseWithClaims(token, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(signingKey), nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := parsed.Claims.(*Claims)
	if !ok || !parsed.Valid {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}
