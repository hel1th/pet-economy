package app_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hel1th/pet-economy/pet/app"
)

func TestLeaderboardHandlerComputesRankOnlyWhenNeeded(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		query         app.LeaderboardQuery
		wantRankCalls int
		wantMyRank    bool
	}{
		{name: "plain page skips rank", query: app.LeaderboardQuery{}, wantRankCalls: 0},
		{
			name:  "explicit flag computes rank",
			query: app.LeaderboardQuery{WithMyRank: true}, wantRankCalls: 1, wantMyRank: true,
		},
		{
			name:  "around me computes rank",
			query: app.LeaderboardQuery{Around: true}, wantRankCalls: 1, wantMyRank: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			read := &stubLeaderboardRead{entries: makeEntries(2), rank: 7, hasRank: true}
			handler := app.NewLeaderboardHandler(read)

			query := tt.query
			query.UserID = uuid.New()

			result, err := handler.Handle(context.Background(), query)
			require.NoError(t, err)

			assert.Equal(t, tt.wantRankCalls, read.rankCalls)

			if tt.wantMyRank {
				require.NotNil(t, result.MyRank)
				assert.Equal(t, 7, *result.MyRank)
			} else {
				assert.Nil(t, result.MyRank)
			}
		})
	}
}

func TestLeaderboardHandlerIgnoresRankErrorWhenRankNotRequested(t *testing.T) {
	t.Parallel()

	read := &stubLeaderboardRead{entries: makeEntries(2), rankErr: errors.New("rank exploded")}
	handler := app.NewLeaderboardHandler(read)

	result, err := handler.Handle(context.Background(), app.LeaderboardQuery{UserID: uuid.New()})

	require.NoError(t, err)
	assert.Len(t, result.Items, 2)
	assert.Zero(t, read.rankCalls)
}
