package app

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/hel1th/pet-economy/shared/pagination"
)

const neighborsRadius = 3

type LeaderboardEntry struct {
	UserID     uuid.UUID
	Name       string
	Level      int
	XP         int
	StreakDays int
	Rank       int
}

type LeaderboardFilter struct {
	Cursor pagination.LeaderboardCursor
	Limit  int
	Offset int
}

type LeaderboardReadModel interface {
	Page(ctx context.Context, f LeaderboardFilter) ([]LeaderboardEntry, error)
	RankOf(ctx context.Context, userID uuid.UUID) (int, bool, error)
}

type LeaderboardQuery struct {
	UserID     uuid.UUID
	Cursor     pagination.LeaderboardCursor
	Limit      int
	Around     bool
	WithMyRank bool
}

type LeaderboardResult struct {
	Items      []LeaderboardEntry
	MyRank     *int
	NextCursor string
}

type LeaderboardHandler struct {
	read LeaderboardReadModel
}

func NewLeaderboardHandler(read LeaderboardReadModel) *LeaderboardHandler {
	return &LeaderboardHandler{read: read}
}

func (h *LeaderboardHandler) Handle(ctx context.Context, q LeaderboardQuery) (LeaderboardResult, error) {
	limit := pagination.NormalizeLimit(q.Limit)

	var (
		myRank  int
		hasRank bool
		err     error
	)

	if q.Around || q.WithMyRank {
		myRank, hasRank, err = h.read.RankOf(ctx, q.UserID)
		if err != nil {
			return LeaderboardResult{}, fmt.Errorf("rank of user: %w", err)
		}
	}

	filter := LeaderboardFilter{Cursor: q.Cursor, Limit: limit + 1}
	if q.Around && hasRank {
		filter = aroundFilter(myRank, limit)
	}

	rows, err := h.read.Page(ctx, filter)
	if err != nil {
		return LeaderboardResult{}, fmt.Errorf("leaderboard page: %w", err)
	}

	result := LeaderboardResult{Items: rows}
	if hasRank {
		rank := myRank
		result.MyRank = &rank
	}

	if len(rows) > limit {
		last := rows[limit-1]
		result.NextCursor = pagination.LeaderboardCursor{
			Level: last.Level, XP: last.XP, UserID: last.UserID,
		}.Encode()
		result.Items = rows[:limit]
	}

	return result, nil
}

func aroundFilter(myRank, limit int) LeaderboardFilter {
	window := neighborsRadius*2 + 1
	if window > limit {
		window = limit
	}

	offset := myRank - 1 - neighborsRadius
	if offset < 0 {
		offset = 0
	}

	return LeaderboardFilter{Limit: window, Offset: offset}
}
