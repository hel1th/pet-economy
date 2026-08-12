package domain

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRewardItemPublishedRespectsDailyLimit(t *testing.T) {
	t.Parallel()

	pet := neutralPet()
	action := LimitedAction{SubjectID: uuid.New(), Unique: true, RewardedCount: 0, Now: testTime()}

	progress, err := pet.RewardItemPublished(action)
	require.NoError(t, err)
	assert.Positive(t, progress.XPGranted)
	assert.Equal(t, 70, pet.Satiety())

	action.RewardedCount = 3
	action.SubjectID = uuid.New()

	_, err = pet.RewardItemPublished(action)
	require.ErrorIs(t, err, ErrLimitReached)
}

func TestRewardItemPublishedRejections(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		action  LimitedAction
		wantErr error
	}{
		{
			name:    "nil subject",
			action:  LimitedAction{Unique: true, Now: testTime()},
			wantErr: ErrInvalidAction,
		},
		{
			name:    "negative rewarded count",
			action:  LimitedAction{SubjectID: uuid.New(), Unique: true, RewardedCount: -1, Now: testTime()},
			wantErr: ErrInvalidAction,
		},
		{
			name:    "duplicate subject",
			action:  LimitedAction{SubjectID: uuid.New(), Unique: false, Now: testTime()},
			wantErr: ErrDuplicateAction,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			pet := neutralPet()

			_, err := pet.RewardItemPublished(tc.action)

			require.ErrorIs(t, err, tc.wantErr)
			assert.Zero(t, pet.XP())
		})
	}
}

func TestRewardItemSold(t *testing.T) {
	t.Parallel()

	pet := neutralPet()
	action := LimitedAction{SubjectID: uuid.New(), Unique: true, Now: testTime()}

	progress, err := pet.RewardItemSold(action)
	require.NoError(t, err)
	assert.Positive(t, progress.XPGranted)
	assert.Equal(t, 90, pet.Happiness())

	_, err = pet.RewardItemSold(LimitedAction{SubjectID: uuid.New(), Unique: false, Now: testTime()})
	require.ErrorIs(t, err, ErrDuplicateAction)

	_, err = pet.RewardItemSold(LimitedAction{Unique: true, Now: testTime()})
	require.ErrorIs(t, err, ErrInvalidAction)
}

func TestRewardItemSoldRespectsDailyLimit(t *testing.T) {
	t.Parallel()

	pet := neutralPet()

	for count := range MaxSoldPerDay {
		_, err := pet.RewardItemSold(LimitedAction{
			SubjectID: uuid.New(), Unique: true, RewardedCount: count, Now: testTime(),
		})
		require.NoError(t, err)
	}

	assert.Positive(t, pet.XP())

	_, err := pet.RewardItemSold(LimitedAction{
		SubjectID: uuid.New(), Unique: true, RewardedCount: MaxSoldPerDay, Now: testTime(),
	})
	require.ErrorIs(t, err, ErrLimitReached)
}
