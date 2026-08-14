package auth_test

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hel1th/pet-economy/shared/auth"
)

const testSecret = "test-secret-value"

var fixedNow = time.Date(2026, time.March, 10, 9, 0, 0, 0, time.UTC)

func frozenClock() func() time.Time {
	return func() time.Time { return fixedNow }
}

func TestIssueAndParseRoundTrip(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		role auth.Role
	}{
		{name: "user", role: auth.RoleUser},
		{name: "moderator", role: auth.RoleModerator},
		{name: "admin", role: auth.RoleAdmin},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc := auth.NewTokenService(testSecret, time.Hour, frozenClock())
			actor := auth.Actor{ID: uuid.New(), Role: tt.role}

			token, expiresAt, err := svc.Issue(actor)
			require.NoError(t, err)
			require.NotEmpty(t, token)
			assert.Equal(t, fixedNow.Add(time.Hour), expiresAt)

			parsed, err := svc.Parse(token)
			require.NoError(t, err)
			assert.Equal(t, actor.ID, parsed.ID)
			assert.Equal(t, tt.role, parsed.Role)
		})
	}
}

func TestParseExpiredToken(t *testing.T) {
	t.Parallel()

	issuer := auth.NewTokenService(testSecret, time.Minute, frozenClock())

	token, _, err := issuer.Issue(auth.Actor{ID: uuid.New(), Role: auth.RoleUser})
	require.NoError(t, err)

	later := fixedNow.Add(2 * time.Minute)
	verifier := auth.NewTokenService(testSecret, time.Minute, func() time.Time { return later })

	_, err = verifier.Parse(token)

	require.ErrorIs(t, err, auth.ErrExpiredToken)
}

func TestParseTokenSignedWithOtherSecret(t *testing.T) {
	t.Parallel()

	forged := auth.NewTokenService("attacker-secret", time.Hour, frozenClock())

	token, _, err := forged.Issue(auth.Actor{ID: uuid.New(), Role: auth.RoleAdmin})
	require.NoError(t, err)

	verifier := auth.NewTokenService(testSecret, time.Hour, frozenClock())

	_, err = verifier.Parse(token)

	require.ErrorIs(t, err, auth.ErrInvalidToken)
}

func TestParseRejectsAlgNone(t *testing.T) {
	t.Parallel()

	claims := auth.Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   uuid.NewString(),
			IssuedAt:  jwt.NewNumericDate(fixedNow),
			ExpiresAt: jwt.NewNumericDate(fixedNow.Add(time.Hour)),
		},
		Role: auth.RoleAdmin,
	}

	unsigned, err := jwt.NewWithClaims(jwt.SigningMethodNone, claims).
		SignedString(jwt.UnsafeAllowNoneSignatureType)
	require.NoError(t, err)

	svc := auth.NewTokenService(testSecret, time.Hour, frozenClock())

	_, err = svc.Parse(unsigned)

	require.ErrorIs(t, err, auth.ErrInvalidToken)
}

func TestParseMalformedTokens(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
	}{
		{name: "empty", raw: ""},
		{name: "not a jwt", raw: "garbage"},
		{name: "two segments", raw: "header.payload"},
		{name: "tampered signature", raw: "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJ4In0.bad"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc := auth.NewTokenService(testSecret, time.Hour, frozenClock())

			_, err := svc.Parse(tt.raw)

			require.ErrorIs(t, err, auth.ErrInvalidToken)
		})
	}
}

func TestParseRejectsBadClaims(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		subject string
		role    auth.Role
	}{
		{name: "non uuid subject", subject: "not-a-uuid", role: auth.RoleUser},
		{name: "empty subject", subject: "", role: auth.RoleUser},
		{name: "unknown role", subject: uuid.NewString(), role: auth.Role("superuser")},
		{name: "empty role", subject: uuid.NewString(), role: auth.Role("")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			claims := auth.Claims{
				RegisteredClaims: jwt.RegisteredClaims{
					Subject:   tt.subject,
					IssuedAt:  jwt.NewNumericDate(fixedNow),
					ExpiresAt: jwt.NewNumericDate(fixedNow.Add(time.Hour)),
				},
				Role: tt.role,
			}

			token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(testSecret))
			require.NoError(t, err)

			svc := auth.NewTokenService(testSecret, time.Hour, frozenClock())

			_, err = svc.Parse(token)

			require.ErrorIs(t, err, auth.ErrInvalidToken)
		})
	}
}

func TestNewTokenServiceDefaultsClock(t *testing.T) {
	t.Parallel()

	svc := auth.NewTokenService(testSecret, time.Hour, nil)

	token, expiresAt, err := svc.Issue(auth.Actor{ID: uuid.New(), Role: auth.RoleUser})

	require.NoError(t, err)
	require.NotEmpty(t, token)
	assert.WithinDuration(t, time.Now().Add(time.Hour), expiresAt, time.Minute)
}

func TestIssueProducesUniqueTokenIDs(t *testing.T) {
	t.Parallel()

	svc := auth.NewTokenService(testSecret, time.Hour, frozenClock())
	actor := auth.Actor{ID: uuid.New(), Role: auth.RoleUser}

	first, _, err := svc.Issue(actor)
	require.NoError(t, err)

	second, _, err := svc.Issue(actor)
	require.NoError(t, err)

	assert.NotEqual(t, first, second)
}
