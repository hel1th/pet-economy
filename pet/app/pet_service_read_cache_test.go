package app_test

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hel1th/pet-economy/pet/app"
	"github.com/hel1th/pet-economy/shared/domainerr"
)

func TestReadPetCachesRepositoryResultOnCacheMiss(t *testing.T) {
	t.Parallel()

	cache := newFakeCache()
	repository := newFakeRepository()
	service := app.NewService(repository, newFakeJournal(), cache, passthroughTx{}, fixedClock{now: testNow()})
	userID := uuid.New()

	_, err := service.Create(t.Context(), userID)
	require.NoError(t, err)

	delete(cache.stored, userID)
	cache.hits = 0

	_, err = service.State(t.Context(), userID)
	require.NoError(t, err)

	assert.Contains(t, cache.stored, userID)
	assert.Zero(t, cache.hits)

	_, err = service.State(t.Context(), userID)
	require.NoError(t, err)

	assert.Equal(t, 1, cache.hits)
}

func TestReadPetSkipsCachingWhenRepositoryMisses(t *testing.T) {
	t.Parallel()

	cache := newFakeCache()
	service := app.NewService(
		newFakeRepository(), newFakeJournal(), cache, passthroughTx{}, fixedClock{now: testNow()})

	_, err := service.State(t.Context(), uuid.New())

	require.ErrorIs(t, err, domainerr.ErrNotFound)
	assert.Empty(t, cache.stored)
}

func TestReadPetToleratesCacheWriteFailureOnMiss(t *testing.T) {
	t.Parallel()

	cache := &brokenCache{getErr: errors.New("redis refused"), setErr: errors.New("redis full")}
	repository := newFakeRepository()
	service := app.NewService(repository, newFakeJournal(), cache, passthroughTx{}, fixedClock{now: testNow()})
	userID := uuid.New()

	_, err := service.Create(t.Context(), userID)
	require.NoError(t, err)

	pet, err := service.State(t.Context(), userID)

	require.NoError(t, err)
	assert.Equal(t, userID, pet.UserID())
}
