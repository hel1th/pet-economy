package pagination_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hel1th/pet-economy/shared/pagination"
)

type row struct {
	id        uuid.UUID
	createdAt time.Time
}

func rowKey(r row) pagination.CursorKey {
	return pagination.CursorKey{CreatedAt: r.createdAt, ID: r.id}
}

func makeRows(n int, base time.Time) []row {
	rows := make([]row, 0, n)
	for i := range n {
		rows = append(rows, row{id: uuid.New(), createdAt: base.Add(-time.Duration(i) * time.Minute)})
	}

	return rows
}

func TestPaginateReturnsEmptyCursorWhenNoOverflow(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, time.March, 1, 12, 0, 0, 0, time.UTC)
	rows := makeRows(3, base)

	page, next := pagination.Paginate(rows, 5, rowKey)

	assert.Len(t, page, 3)
	assert.Empty(t, next)
}

func TestPaginateReturnsEmptyCursorOnExactFit(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, time.March, 1, 12, 0, 0, 0, time.UTC)
	rows := makeRows(4, base)

	page, next := pagination.Paginate(rows, 4, rowKey)

	assert.Len(t, page, 4)
	assert.Empty(t, next)
}

func TestPaginateTrimsOverflowAndEncodesLastKeptRow(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, time.March, 1, 12, 0, 0, 0, time.UTC)
	rows := makeRows(4, base)

	page, next := pagination.Paginate(rows, 3, rowKey)

	require.Len(t, page, 3)
	require.NotEmpty(t, next)

	decoded, err := pagination.DecodeCursor(next)
	require.NoError(t, err)

	assert.Equal(t, rows[2].id, decoded.ID)
	assert.True(t, rows[2].createdAt.Equal(decoded.CreatedAt))
}

func TestPaginateWithNonPositiveLimitPassesThrough(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, time.March, 1, 12, 0, 0, 0, time.UTC)
	rows := makeRows(2, base)

	page, next := pagination.Paginate(rows, 0, rowKey)

	assert.Len(t, page, 2)
	assert.Empty(t, next)
}

func TestPaginateWithEmptyInput(t *testing.T) {
	t.Parallel()

	page, next := pagination.Paginate([]row{}, 10, rowKey)

	assert.Empty(t, page)
	assert.Empty(t, next)
}
