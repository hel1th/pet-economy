package domainerr

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSentinelErrorsAreDistinct(t *testing.T) {
	t.Parallel()

	sentinels := []error{ErrNotFound, ErrForbidden, ErrUnauthorized, ErrConflict}

	for i, outer := range sentinels {
		for j, inner := range sentinels {
			if i == j {
				continue
			}

			require.NotErrorIs(t, outer, inner)
		}
	}
}

func TestInvalidError(t *testing.T) {
	t.Parallel()

	err := NewInvalid("email", "must not be empty")

	require.Error(t, err)
	assert.Equal(t, "email", err.Field)
	assert.Equal(t, "must not be empty", err.Message)
	assert.Equal(t, "email: must not be empty", err.Error())
}

func TestInvalidErrorUnwrapsThroughWrapping(t *testing.T) {
	t.Parallel()

	err := NewInvalid("price", "must be positive")
	wrapped := fmt.Errorf("create item: %w", err)

	var target *InvalidError

	require.ErrorAs(t, wrapped, &target)
	assert.Equal(t, "price", target.Field)
}

func TestConflictError(t *testing.T) {
	t.Parallel()

	err := NewConflict("email already taken")

	require.Error(t, err)
	assert.Equal(t, "email already taken", err.Error())
}

func TestConflictErrorIsErrConflict(t *testing.T) {
	t.Parallel()

	err := NewConflict("duplicate")

	require.ErrorIs(t, err, ErrConflict)
}

func TestConflictErrorIsNotOtherSentinels(t *testing.T) {
	t.Parallel()

	err := NewConflict("duplicate")

	require.NotErrorIs(t, err, ErrNotFound)
	require.NotErrorIs(t, err, ErrForbidden)
	require.NotErrorIs(t, err, ErrUnauthorized)
}

func TestConflictErrorMatchesThroughWrapping(t *testing.T) {
	t.Parallel()

	wrapped := fmt.Errorf("register user: %w", NewConflict("duplicate"))

	require.ErrorIs(t, wrapped, ErrConflict)

	var target *ConflictError

	require.ErrorAs(t, wrapped, &target)
	assert.Equal(t, "duplicate", target.Message)
}
