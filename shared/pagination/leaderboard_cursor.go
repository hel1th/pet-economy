package pagination

import (
	"strconv"
	"strings"

	"github.com/google/uuid"
)

const leaderboardCursorParts = 3

type LeaderboardCursor struct {
	Level  int
	XP     int
	UserID uuid.UUID
}

func (c LeaderboardCursor) IsZero() bool {
	return c.UserID == uuid.Nil
}

func (c LeaderboardCursor) Encode() string {
	if c.IsZero() {
		return ""
	}

	raw := strconv.Itoa(c.Level) + "|" + strconv.Itoa(c.XP) + "|" + c.UserID.String()

	return sealCursor(raw)
}

func DecodeLeaderboardCursor(raw string) (LeaderboardCursor, error) {
	if raw == "" {
		return LeaderboardCursor{}, nil
	}

	decoded, err := openCursor(raw)
	if err != nil {
		return LeaderboardCursor{}, ErrInvalidCursor
	}

	parts := strings.Split(decoded, "|")
	if len(parts) != leaderboardCursorParts {
		return LeaderboardCursor{}, ErrInvalidCursor
	}

	level, err := strconv.Atoi(parts[0])
	if err != nil {
		return LeaderboardCursor{}, ErrInvalidCursor
	}

	xp, err := strconv.Atoi(parts[1])
	if err != nil {
		return LeaderboardCursor{}, ErrInvalidCursor
	}

	userID, err := uuid.Parse(parts[2])
	if err != nil || userID == uuid.Nil {
		return LeaderboardCursor{}, ErrInvalidCursor
	}

	return LeaderboardCursor{Level: level, XP: xp, UserID: userID}, nil
}
