package pagination

import (
	"time"

	"github.com/google/uuid"
)

type CursorKey struct {
	CreatedAt time.Time
	ID        uuid.UUID
}

func Paginate[T any](rows []T, limit int, key func(T) CursorKey) (page []T, next string) {
	if limit <= 0 || len(rows) <= limit {
		return rows, ""
	}

	page = rows[:limit]
	last := key(page[limit-1])

	return page, Cursor{CreatedAt: last.CreatedAt, ID: last.ID}.Encode()
}
