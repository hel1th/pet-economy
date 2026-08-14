package domain_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hel1th/pet-economy/pet/domain"
)

func TestRewardItemUpdated(t *testing.T) {
	t.Parallel()
	pet := domain.New(uuid.New(), time.Now())
	action := domain.LimitedAction{SubjectID: uuid.New(), Unique: true, RewardedCount: 0, Now: time.Now()}

	progress, err := pet.RewardItemUpdated(action)
	require.NoError(t, err)
	assert.Equal(t, 6, progress.XPGranted)
}

func TestRewardItemImproved(t *testing.T) {
	t.Parallel()
	pet := domain.New(uuid.New(), time.Now())
	action := domain.LimitedAction{SubjectID: uuid.New(), Unique: true, RewardedCount: 0, Now: time.Now()}

	improvement := domain.ListingImprovement{PhotoAdded: false, DescriptionAdded: false, PriceSet: false}
	assert.False(t, improvement.Any())
	_, err := pet.RewardItemImproved(action, improvement)
	require.ErrorIs(t, err, domain.ErrConditionNotMet)

	improvement.PhotoAdded = true
	assert.True(t, improvement.Any())
	progress, err := pet.RewardItemImproved(action, improvement)
	require.NoError(t, err)
	assert.Equal(t, 19, progress.XPGranted)
}
