package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

var (
	ErrInvalidToken = errors.New("invalid token")
	ErrExpiredToken = errors.New("token expired")
)

type Claims struct {
	jwt.RegisteredClaims
	Role Role `json:"role"`
}

type TokenService struct {
	secret []byte
	ttl    time.Duration
	now    func() time.Time
}

func NewTokenService(secret string, ttl time.Duration, now func() time.Time) *TokenService {
	if now == nil {
		now = time.Now
	}

	return &TokenService{secret: []byte(secret), ttl: ttl, now: now}
}

func (s *TokenService) Issue(actor Actor) (string, time.Time, error) {
	issuedAt := s.now()
	expiresAt := issuedAt.Add(s.ttl)

	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   actor.ID.String(),
			IssuedAt:  jwt.NewNumericDate(issuedAt),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			ID:        uuid.NewString(),
		},
		Role: actor.Role,
	}

	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign token: %w", err)
	}

	return signed, expiresAt, nil
}

func (s *TokenService) Parse(raw string) (Actor, error) {
	claims := &Claims{}

	token, err := jwt.ParseWithClaims(raw, claims, s.keyFunc,
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithTimeFunc(s.now),
	)
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return Actor{}, ErrExpiredToken
		}

		return Actor{}, ErrInvalidToken
	}

	if !token.Valid {
		return Actor{}, ErrInvalidToken
	}

	id, err := uuid.Parse(claims.Subject)
	if err != nil {
		return Actor{}, ErrInvalidToken
	}

	if !claims.Role.Valid() {
		return Actor{}, ErrInvalidToken
	}

	return Actor{ID: id, Role: claims.Role}, nil
}

func (s *TokenService) keyFunc(token *jwt.Token) (any, error) {
	if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
		return nil, ErrInvalidToken
	}

	return s.secret, nil
}
