package app_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hel1th/pet-economy/pet/app"
	"github.com/hel1th/pet-economy/shared/pagination"
)

type stubLeaderboardRead struct {
	entries    []app.LeaderboardEntry
	lastFilter app.LeaderboardFilter
	rank       int
	hasRank    bool
	rankCalls  int
	pageErr    error
	rankErr    error
}

func (s *stubLeaderboardRead) Page(_ context.Context, f app.LeaderboardFilter) ([]app.LeaderboardEntry, error) {
	s.lastFilter = f
	if s.pageErr != nil {
		return nil, s.pageErr
	}

	return s.entries, nil
}

func (s *stubLeaderboardRead) RankOf(_ context.Context, _ uuid.UUID) (rank int, found bool, err error) {
	s.rankCalls++
	if s.rankErr != nil {
		return 0, false, s.rankErr
	}

	return s.rank, s.hasRank, nil
}

func makeEntries(n int) []app.LeaderboardEntry {
	entries := make([]app.LeaderboardEntry, 0, n)
	for i := range n {
		entries = append(entries, app.LeaderboardEntry{
			UserID: uuid.New(), Level: 100 - i, XP: 500 - i, Rank: i + 1,
		})
	}

	return entries
}

func TestLeaderboardHandlerPagination(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		rows         int
		limit        int
		wantItems    int
		wantNextPage bool
	}{
		{name: "empty board", rows: 0, limit: 20, wantItems: 0},
		{name: "partial page", rows: 5, limit: 20, wantItems: 5},
		{name: "exactly one page", rows: 20, limit: 20, wantItems: 20},
		{name: "has next page", rows: 21, limit: 20, wantItems: 20, wantNextPage: true},
		{name: "zero limit falls back to default", rows: 21, limit: 0, wantItems: 20, wantNextPage: true},
		{name: "limit above max is clamped", rows: 101, limit: 5000, wantItems: 100, wantNextPage: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			read := &stubLeaderboardRead{entries: makeEntries(tt.rows)}
			handler := app.NewLeaderboardHandler(read)

			result, err := handler.Handle(context.Background(), app.LeaderboardQuery{
				UserID: uuid.New(), Limit: tt.limit,
			})
			require.NoError(t, err)

			assert.Len(t, result.Items, tt.wantItems)
			assert.Nil(t, result.MyRank)

			if tt.wantNextPage {
				assert.NotEmpty(t, result.NextCursor)
			} else {
				assert.Empty(t, result.NextCursor)
			}
		})
	}
}

func TestLeaderboardHandlerRequestsOneExtraRow(t *testing.T) {
	t.Parallel()

	read := &stubLeaderboardRead{entries: makeEntries(3)}
	handler := app.NewLeaderboardHandler(read)

	_, err := handler.Handle(context.Background(), app.LeaderboardQuery{UserID: uuid.New(), Limit: 10})
	require.NoError(t, err)

	assert.Equal(t, 11, read.lastFilter.Limit)
	assert.Equal(t, 0, read.lastFilter.Offset)
}

func TestLeaderboardHandlerNextCursorPointsAtLastReturnedRow(t *testing.T) {
	t.Parallel()

	entries := makeEntries(3)
	read := &stubLeaderboardRead{entries: entries}
	handler := app.NewLeaderboardHandler(read)

	result, err := handler.Handle(context.Background(), app.LeaderboardQuery{UserID: uuid.New(), Limit: 2})
	require.NoError(t, err)
	require.Len(t, result.Items, 2)

	decoded, err := pagination.DecodeLeaderboardCursor(result.NextCursor)
	require.NoError(t, err)

	last := entries[1]
	assert.Equal(t, last.UserID, decoded.UserID)
	assert.Equal(t, last.Level, decoded.Level)
	assert.Equal(t, last.XP, decoded.XP)
}

func TestLeaderboardHandlerMyRank(t *testing.T) {
	t.Parallel()

	t.Run("present when requested and user has a pet", func(t *testing.T) {
		t.Parallel()

		read := &stubLeaderboardRead{entries: makeEntries(2), rank: 42, hasRank: true}
		handler := app.NewLeaderboardHandler(read)

		result, err := handler.Handle(context.Background(), app.LeaderboardQuery{
			UserID: uuid.New(), WithMyRank: true,
		})
		require.NoError(t, err)
		require.NotNil(t, result.MyRank)
		assert.Equal(t, 42, *result.MyRank)
	})

	t.Run("nil when requested but user has no pet", func(t *testing.T) {
		t.Parallel()

		read := &stubLeaderboardRead{entries: makeEntries(2)}
		handler := app.NewLeaderboardHandler(read)

		result, err := handler.Handle(context.Background(), app.LeaderboardQuery{
			UserID: uuid.New(), WithMyRank: true,
		})
		require.NoError(t, err)
		assert.Nil(t, result.MyRank)
	})
}

func TestLeaderboardHandlerAroundMe(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		rank       int
		hasRank    bool
		limit      int
		wantOffset int
		wantLimit  int
	}{
		{name: "middle of the board", rank: 50, hasRank: true, limit: 20, wantOffset: 46, wantLimit: 7},
		{name: "first place clamps offset", rank: 1, hasRank: true, limit: 20, wantOffset: 0, wantLimit: 7},
		{name: "third place clamps offset", rank: 3, hasRank: true, limit: 20, wantOffset: 0, wantLimit: 7},
		{name: "small limit shrinks window", rank: 50, hasRank: true, limit: 3, wantOffset: 46, wantLimit: 3},
		{name: "no pet falls back to keyset", rank: 0, hasRank: false, limit: 20, wantOffset: 0, wantLimit: 21},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			read := &stubLeaderboardRead{entries: makeEntries(3), rank: tt.rank, hasRank: tt.hasRank}
			handler := app.NewLeaderboardHandler(read)

			_, err := handler.Handle(context.Background(), app.LeaderboardQuery{
				UserID: uuid.New(), Limit: tt.limit, Around: true,
			})
			require.NoError(t, err)

			assert.Equal(t, tt.wantOffset, read.lastFilter.Offset)
			assert.Equal(t, tt.wantLimit, read.lastFilter.Limit)
		})
	}
}

func TestLeaderboardHandlerPassesCursorThrough(t *testing.T) {
	t.Parallel()

	cursor := pagination.LeaderboardCursor{Level: 9, XP: 77, UserID: uuid.New()}
	read := &stubLeaderboardRead{entries: makeEntries(1)}
	handler := app.NewLeaderboardHandler(read)

	_, err := handler.Handle(context.Background(), app.LeaderboardQuery{UserID: uuid.New(), Cursor: cursor})
	require.NoError(t, err)

	assert.Equal(t, cursor, read.lastFilter.Cursor)
}

func TestLeaderboardHandlerErrors(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("boom")

	t.Run("page error", func(t *testing.T) {
		t.Parallel()

		handler := app.NewLeaderboardHandler(&stubLeaderboardRead{pageErr: sentinel})

		_, err := handler.Handle(context.Background(), app.LeaderboardQuery{UserID: uuid.New()})
		require.ErrorIs(t, err, sentinel)
	})

	t.Run("rank error", func(t *testing.T) {
		t.Parallel()

		handler := app.NewLeaderboardHandler(&stubLeaderboardRead{rankErr: sentinel})

		_, err := handler.Handle(context.Background(), app.LeaderboardQuery{
			UserID: uuid.New(), WithMyRank: true,
		})
		require.ErrorIs(t, err, sentinel)
	})
}
