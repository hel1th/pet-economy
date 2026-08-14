package pagination_test

import (
	"encoding/base64"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hel1th/pet-economy/shared/pagination"
)

func TestLeaderboardCursorRoundTrip(t *testing.T) {
	t.Parallel()

	userID := uuid.MustParse("11111111-2222-3333-4444-555555555555")

	tests := []struct {
		name   string
		cursor pagination.LeaderboardCursor
	}{
		{name: "typical", cursor: pagination.LeaderboardCursor{Level: 7, XP: 420, UserID: userID}},
		{name: "zero values", cursor: pagination.LeaderboardCursor{Level: 0, XP: 0, UserID: userID}},
		{name: "min level", cursor: pagination.LeaderboardCursor{Level: 1, XP: 0, UserID: userID}},
		{name: "large xp", cursor: pagination.LeaderboardCursor{Level: 999, XP: 2147483647, UserID: userID}},
		{name: "negative level", cursor: pagination.LeaderboardCursor{Level: -3, XP: -1, UserID: userID}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			encoded := tt.cursor.Encode()
			require.NotEmpty(t, encoded)
			assert.NotContains(t, encoded, "=")

			decoded, err := pagination.DecodeLeaderboardCursor(encoded)
			require.NoError(t, err)
			assert.Equal(t, tt.cursor, decoded)
		})
	}
}

func TestLeaderboardCursorIsZero(t *testing.T) {
	t.Parallel()

	assert.True(t, pagination.LeaderboardCursor{}.IsZero())
	assert.True(t, pagination.LeaderboardCursor{Level: 5, XP: 10}.IsZero())
	assert.False(t, pagination.LeaderboardCursor{UserID: uuid.New()}.IsZero())
}

func TestLeaderboardCursorEncodeZeroIsEmpty(t *testing.T) {
	t.Parallel()

	assert.Empty(t, pagination.LeaderboardCursor{}.Encode())
	assert.Empty(t, pagination.LeaderboardCursor{Level: 3, XP: 4}.Encode())
}

func TestDecodeLeaderboardCursor(t *testing.T) {
	t.Parallel()

	encode := func(raw string) string {
		return base64.RawURLEncoding.EncodeToString([]byte(raw))
	}

	tests := []struct {
		name    string
		raw     string
		wantErr bool
		want    pagination.LeaderboardCursor
	}{
		{name: "empty means first page", raw: "", want: pagination.LeaderboardCursor{}},
		{name: "not base64", raw: "!!!not base64!!!", wantErr: true},
		{name: "unencrypted too few parts", raw: encode("5|100"), wantErr: true},
		{name: "unencrypted too many parts", raw: encode("5|100|" + uuid.Nil.String() + "|extra"), wantErr: true},
		{name: "unencrypted level not a number", raw: encode("abc|100|" + uuid.New().String()), wantErr: true},
		{name: "unencrypted xp not a number", raw: encode("5|abc|" + uuid.New().String()), wantErr: true},
		{name: "unencrypted malformed uuid", raw: encode("5|100|not-a-uuid"), wantErr: true},
		{name: "unencrypted nil uuid rejected", raw: encode("5|100|" + uuid.Nil.String()), wantErr: true},
		{name: "unencrypted blank payload", raw: encode("||"), wantErr: true},
		{name: "unencrypted empty fields", raw: encode("5||" + uuid.New().String()), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := pagination.DecodeLeaderboardCursor(tt.raw)
			if tt.wantErr {
				require.ErrorIs(t, err, pagination.ErrInvalidCursor)
				assert.True(t, got.IsZero())

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
