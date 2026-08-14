package pagination

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	DefaultLimit = 20
	MaxLimit     = 100
)

const (
	cursorMinParts = 2
	cursorMaxParts = 4
)

var ErrInvalidCursor = errors.New("invalid cursor")

type Cursor struct {
	CreatedAt   time.Time
	ID          uuid.UUID
	PriceKopeks int64
	Sort        string
}

func (c Cursor) IsZero() bool {
	return c.ID == uuid.Nil
}

func (c Cursor) Encode() string {
	if c.IsZero() {
		return ""
	}

	raw := fmt.Sprintf("%s|%s|%s|%s",
		c.CreatedAt.UTC().Format(time.RFC3339Nano), c.ID,
		strconv.FormatInt(c.PriceKopeks, 10), c.Sort)

	return sealCursor(raw)
}

func DecodeCursor(raw string) (Cursor, error) {
	if raw == "" {
		return Cursor{}, nil
	}

	decoded, err := openCursor(raw)
	if err != nil {
		return Cursor{}, ErrInvalidCursor
	}

	parts := strings.Split(decoded, "|")
	if len(parts) < cursorMinParts || len(parts) > cursorMaxParts {
		return Cursor{}, ErrInvalidCursor
	}

	createdAt, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return Cursor{}, ErrInvalidCursor
	}

	id, err := uuid.Parse(parts[1])
	if err != nil || id == uuid.Nil {
		return Cursor{}, ErrInvalidCursor
	}

	cursor := Cursor{CreatedAt: createdAt, ID: id}

	if len(parts) > cursorMinParts {
		price, err := strconv.ParseInt(parts[2], 10, 64)
		if err != nil {
			return Cursor{}, ErrInvalidCursor
		}

		cursor.PriceKopeks = price
	}

	if len(parts) == cursorMaxParts {
		cursor.Sort = parts[3]
	}

	return cursor, nil
}

func NormalizeLimit(limit int) int {
	if limit <= 0 {
		return DefaultLimit
	}
	if limit > MaxLimit {
		return MaxLimit
	}

	return limit
}
