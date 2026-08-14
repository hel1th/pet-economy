package pagination_test

import (
	"encoding/base64"
	"math"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hel1th/pet-economy/shared/pagination"
)

func TestCursorRoundTrip(t *testing.T) {
	t.Parallel()

	original := pagination.Cursor{
		CreatedAt: time.Date(2026, time.February, 3, 14, 25, 36, 123456789, time.UTC),
		ID:        uuid.New(),
	}

	decoded, err := pagination.DecodeCursor(original.Encode())

	require.NoError(t, err)
	assert.True(t, original.CreatedAt.Equal(decoded.CreatedAt))
	assert.Equal(t, original.ID, decoded.ID)
	assert.False(t, decoded.IsZero())
}

func TestCursorRoundTripPreservesPrice(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		price int64
	}{
		{name: "zero price", price: 0},
		{name: "positive price", price: 1234567},
		{name: "negative price", price: -42},
		{name: "max int64", price: math.MaxInt64},
		{name: "min int64", price: math.MinInt64},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			original := pagination.Cursor{
				CreatedAt:   time.Date(2026, time.February, 3, 14, 25, 36, 123456789, time.UTC),
				ID:          uuid.New(),
				PriceKopeks: tc.price,
			}

			decoded, err := pagination.DecodeCursor(original.Encode())

			require.NoError(t, err)
			assert.True(t, original.CreatedAt.Equal(decoded.CreatedAt))
			assert.Equal(t, original.ID, decoded.ID)
			assert.Equal(t, tc.price, decoded.PriceKopeks)
		})
	}
}

func TestDecodeCursorAcceptsLegacyTwoPartPayload(t *testing.T) {
	t.Parallel()

	id := uuid.New()
	legacy := pagination.SealForTest("2026-02-03T14:00:00Z|" + id.String())

	decoded, err := pagination.DecodeCursor(legacy)

	require.NoError(t, err)
	assert.Equal(t, id, decoded.ID)
	assert.Zero(t, decoded.PriceKopeks)
}

func TestDecodeCursorRejectsMalformedPrice(t *testing.T) {
	t.Parallel()

	id := uuid.New()

	tests := []struct {
		name string
		raw  string
	}{
		{name: "price is not a number", raw: "2026-02-03T14:00:00Z|" + id.String() + "|abc"},
		{name: "price is empty", raw: "2026-02-03T14:00:00Z|" + id.String() + "|"},
		{name: "price overflows int64", raw: "2026-02-03T14:00:00Z|" + id.String() + "|99999999999999999999"},
		{name: "price is a float", raw: "2026-02-03T14:00:00Z|" + id.String() + "|12.5"},
		{name: "too many segments", raw: "2026-02-03T14:00:00Z|" + id.String() + "|1|newest|x"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cursor, err := pagination.DecodeCursor(pagination.SealForTest(tc.raw))

			require.ErrorIs(t, err, pagination.ErrInvalidCursor)
			assert.True(t, cursor.IsZero())
		})
	}
}

func TestDecodeCursorRejectsTamperedPriceCursor(t *testing.T) {
	t.Parallel()

	encoded := pagination.Cursor{
		CreatedAt:   time.Now().UTC(),
		ID:          uuid.New(),
		PriceKopeks: 999,
	}.Encode()

	tampered := flipLastByte(t, encoded)

	cursor, err := pagination.DecodeCursor(tampered)

	require.ErrorIs(t, err, pagination.ErrInvalidCursor)
	assert.True(t, cursor.IsZero())
}

func TestCursorRoundTripsSort(t *testing.T) {
	t.Parallel()

	id := uuid.New()
	created := time.Date(2026, time.March, 1, 9, 0, 0, 0, time.UTC)

	encoded := pagination.Cursor{
		CreatedAt: created, ID: id, PriceKopeks: 4200, Sort: "price_asc",
	}.Encode()

	cursor, err := pagination.DecodeCursor(encoded)

	require.NoError(t, err)
	assert.Equal(t, "price_asc", cursor.Sort)
	assert.Equal(t, int64(4200), cursor.PriceKopeks)
	assert.Equal(t, id, cursor.ID)
	assert.Equal(t, created, cursor.CreatedAt)
}

func TestDecodeCursorAcceptsLegacyCursorWithoutSort(t *testing.T) {
	t.Parallel()

	id := uuid.New()
	raw := "2026-02-03T14:00:00Z|" + id.String()

	cursor, err := pagination.DecodeCursor(pagination.SealForTest(raw))

	require.NoError(t, err)
	assert.Equal(t, id, cursor.ID)
	assert.Empty(t, cursor.Sort)
	assert.Zero(t, cursor.PriceKopeks)
}

func TestCursorNormalizesTimezoneToUTC(t *testing.T) {
	t.Parallel()

	zone := time.FixedZone("MSK", 3*60*60)
	original := pagination.Cursor{
		CreatedAt: time.Date(2026, time.February, 3, 14, 0, 0, 0, zone),
		ID:        uuid.New(),
	}

	decoded, err := pagination.DecodeCursor(original.Encode())

	require.NoError(t, err)
	assert.Equal(t, time.UTC, decoded.CreatedAt.Location())
	assert.True(t, original.CreatedAt.Equal(decoded.CreatedAt))
}

func TestZeroCursorEncodesToEmptyString(t *testing.T) {
	t.Parallel()

	zero := pagination.Cursor{}

	assert.True(t, zero.IsZero())
	assert.Empty(t, zero.Encode())

	withTimeOnly := pagination.Cursor{CreatedAt: time.Now()}
	assert.True(t, withTimeOnly.IsZero())
	assert.Empty(t, withTimeOnly.Encode())
}

func TestDecodeCursorFailures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		raw     string
		wantErr bool
	}{
		{name: "empty string yields zero cursor", raw: ""},
		{name: "not base64", raw: "!!!not-base64!!!", wantErr: true},
		{
			name:    "unencrypted payload without separator",
			raw:     base64.RawURLEncoding.EncodeToString([]byte("2026-02-03T14:00:00Z")),
			wantErr: true,
		},
		{
			name:    "unencrypted invalid timestamp",
			raw:     base64.RawURLEncoding.EncodeToString([]byte("yesterday|" + uuid.NewString())),
			wantErr: true,
		},
		{
			name:    "unencrypted invalid uuid",
			raw:     base64.RawURLEncoding.EncodeToString([]byte("2026-02-03T14:00:00Z|not-a-uuid")),
			wantErr: true,
		},
		{
			name:    "unencrypted nil uuid is rejected",
			raw:     base64.RawURLEncoding.EncodeToString([]byte("2026-02-03T14:00:00Z|" + uuid.Nil.String())),
			wantErr: true,
		},
		{
			name:    "separator without payload",
			raw:     base64.RawURLEncoding.EncodeToString([]byte("|")),
			wantErr: true,
		},
		{
			name:    "padded base64 is rejected",
			raw:     "MjAyNi0wMi0wM1QxNDowMDowMFo=",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cursor, err := pagination.DecodeCursor(tc.raw)

			if tc.wantErr {
				require.ErrorIs(t, err, pagination.ErrInvalidCursor)
				assert.True(t, cursor.IsZero())

				return
			}

			require.NoError(t, err)
			assert.True(t, cursor.IsZero())
		})
	}
}

func TestNormalizeLimit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		limit int
		want  int
	}{
		{name: "zero falls back to default", limit: 0, want: pagination.DefaultLimit},
		{name: "negative falls back to default", limit: -42, want: pagination.DefaultLimit},
		{name: "one stays one", limit: 1, want: 1},
		{name: "value within range is kept", limit: 37, want: 37},
		{name: "max is kept", limit: pagination.MaxLimit, want: pagination.MaxLimit},
		{name: "above max is clamped", limit: pagination.MaxLimit + 1, want: pagination.MaxLimit},
		{name: "huge value is clamped", limit: 1 << 20, want: pagination.MaxLimit},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tc.want, pagination.NormalizeLimit(tc.limit))
		})
	}
}
