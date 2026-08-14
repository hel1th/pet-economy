package pagination_test

import (
	"encoding/base64"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hel1th/pet-economy/shared/pagination"
)

func TestCursorDoesNotLeakIdentifiers(t *testing.T) {
	id := uuid.New()
	cursor := pagination.Cursor{CreatedAt: time.Now().UTC(), ID: id}

	encoded := cursor.Encode()

	require.NotEmpty(t, encoded)
	assert.NotContains(t, encoded, id.String())

	decoded, err := base64.RawURLEncoding.DecodeString(encoded)
	require.NoError(t, err)
	assert.NotContains(t, string(decoded), id.String())
	assert.NotContains(t, string(decoded), cursor.CreatedAt.Format(time.RFC3339Nano))
}

func TestLeaderboardCursorDoesNotLeakIdentifiers(t *testing.T) {
	userID := uuid.New()
	cursor := pagination.LeaderboardCursor{Level: 7, XP: 420, UserID: userID}

	encoded := cursor.Encode()

	require.NotEmpty(t, encoded)
	assert.NotContains(t, encoded, userID.String())

	decoded, err := base64.RawURLEncoding.DecodeString(encoded)
	require.NoError(t, err)
	assert.NotContains(t, string(decoded), userID.String())
}

func TestCursorEncodingIsNonDeterministic(t *testing.T) {
	cursor := pagination.Cursor{CreatedAt: time.Now().UTC(), ID: uuid.New()}

	first := cursor.Encode()
	second := cursor.Encode()

	assert.NotEqual(t, first, second)

	decodedFirst, err := pagination.DecodeCursor(first)
	require.NoError(t, err)

	decodedSecond, err := pagination.DecodeCursor(second)
	require.NoError(t, err)

	assert.Equal(t, decodedFirst.ID, decodedSecond.ID)
}

func TestTamperedCursorIsRejected(t *testing.T) {
	encoded := pagination.Cursor{CreatedAt: time.Now().UTC(), ID: uuid.New()}.Encode()

	tampered := flipLastByte(t, encoded)

	cursor, err := pagination.DecodeCursor(tampered)

	require.ErrorIs(t, err, pagination.ErrInvalidCursor)
	assert.True(t, cursor.IsZero())
}

func TestTamperedLeaderboardCursorIsRejected(t *testing.T) {
	encoded := pagination.LeaderboardCursor{Level: 3, XP: 30, UserID: uuid.New()}.Encode()

	tampered := flipLastByte(t, encoded)

	cursor, err := pagination.DecodeLeaderboardCursor(tampered)

	require.ErrorIs(t, err, pagination.ErrInvalidCursor)
	assert.True(t, cursor.IsZero())
}

func TestForgedPlaintextCursorIsRejected(t *testing.T) {
	forged := base64.RawURLEncoding.EncodeToString(
		[]byte("2026-02-03T14:00:00Z|" + uuid.New().String()),
	)

	cursor, err := pagination.DecodeCursor(forged)

	require.ErrorIs(t, err, pagination.ErrInvalidCursor)
	assert.True(t, cursor.IsZero())
}

func TestForgedPlaintextLeaderboardCursorIsRejected(t *testing.T) {
	forged := base64.RawURLEncoding.EncodeToString(
		[]byte("5|100|" + uuid.New().String()),
	)

	cursor, err := pagination.DecodeLeaderboardCursor(forged)

	require.ErrorIs(t, err, pagination.ErrInvalidCursor)
	assert.True(t, cursor.IsZero())
}

func TestTruncatedCursorIsRejected(t *testing.T) {
	encoded := pagination.Cursor{CreatedAt: time.Now().UTC(), ID: uuid.New()}.Encode()

	cursor, err := pagination.DecodeCursor(encoded[:4])

	require.ErrorIs(t, err, pagination.ErrInvalidCursor)
	assert.True(t, cursor.IsZero())
}

func TestCursorFromAnotherSecretIsRejected(t *testing.T) {
	encoded := pagination.Cursor{CreatedAt: time.Now().UTC(), ID: uuid.New()}.Encode()

	pagination.SetCursorSecret("second-secret-value-000000000")
	defer pagination.SetCursorSecret("")

	cursor, err := pagination.DecodeCursor(encoded)

	require.ErrorIs(t, err, pagination.ErrInvalidCursor)
	assert.True(t, cursor.IsZero())
}

func flipLastByte(t *testing.T, encoded string) string {
	t.Helper()

	raw, err := base64.RawURLEncoding.DecodeString(encoded)
	require.NoError(t, err)
	require.NotEmpty(t, raw)

	raw[len(raw)-1] ^= 0xFF

	return base64.RawURLEncoding.EncodeToString(raw)
}
