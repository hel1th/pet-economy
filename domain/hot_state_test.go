package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplyHotStateAcceptsFreshVersion(t *testing.T) {
	t.Parallel()

	pet := petWithParams(50, 50, 100)
	later := testTime().Add(time.Hour)

	err := pet.ApplyHotState(80, 60, 3, later)

	require.NoError(t, err)
	assert.Equal(t, 80, pet.Happiness())
	assert.Equal(t, 60, pet.Satiety())
	assert.Equal(t, int64(3), pet.InteractionVersion())
	assert.Equal(t, later, pet.UpdatedAt())
}

func TestApplyHotStateKeepsNewerUpdatedAt(t *testing.T) {
	t.Parallel()

	pet := petWithParams(50, 50, 100)
	earlier := testTime().Add(-time.Hour)

	require.NoError(t, pet.ApplyHotState(70, 70, 1, earlier))
	assert.Equal(t, testTime(), pet.UpdatedAt())
}

func TestApplyHotStateRejections(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		happiness int
		satiety   int
		version   int64
	}{
		{name: "happiness below zero", happiness: -1, satiety: 50, version: 1},
		{name: "happiness above max", happiness: 101, satiety: 50, version: 1},
		{name: "satiety below zero", happiness: 50, satiety: -1, version: 1},
		{name: "satiety above max", happiness: 50, satiety: 101, version: 1},
		{name: "negative version", happiness: 50, satiety: 50, version: -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			pet := petWithParams(40, 40, 100)

			err := pet.ApplyHotState(tt.happiness, tt.satiety, tt.version, testTime())

			require.ErrorIs(t, err, ErrInvalidAction)
			assert.Equal(t, 40, pet.Happiness())
			assert.Equal(t, 40, pet.Satiety())
		})
	}
}

func TestApplyHotStateRejectsStaleVersion(t *testing.T) {
	t.Parallel()

	pet := petWithParams(40, 40, 100)
	require.NoError(t, pet.ApplyHotState(60, 60, 5, testTime()))

	err := pet.ApplyHotState(90, 90, 4, testTime())

	require.ErrorIs(t, err, ErrInvalidAction)
	assert.Equal(t, 60, pet.Happiness())
	assert.Equal(t, int64(5), pet.InteractionVersion())
}

func TestApplyHotStateAcceptsSameVersion(t *testing.T) {
	t.Parallel()

	pet := petWithParams(40, 40, 100)
	require.NoError(t, pet.ApplyHotState(60, 60, 5, testTime()))

	require.NoError(t, pet.ApplyHotState(75, 65, 5, testTime()))
	assert.Equal(t, 75, pet.Happiness())
}

func TestNewPetIsAlreadyHatched(t *testing.T) {
	t.Parallel()

	pet := New(petWithParams(50, 50, 100).UserID(), testTime())

	assert.True(t, pet.IsHatched())
	require.NotNil(t, pet.HatchedAt())
	assert.Equal(t, testTime(), *pet.HatchedAt())
	assert.Equal(t, StageBaby, pet.Stage())

	assert.False(t, pet.Hatch(testTime().Add(time.Hour)))
	assert.Equal(t, testTime(), *pet.HatchedAt())
}

func TestHatchIsOneWayForLegacyPets(t *testing.T) {
	t.Parallel()

	pet := legacyUnhatchedPet()
	require.False(t, pet.IsHatched())
	assert.Nil(t, pet.HatchedAt())

	assert.True(t, pet.Hatch(testTime()))
	assert.True(t, pet.IsHatched())
	require.NotNil(t, pet.HatchedAt())
	assert.Equal(t, testTime(), *pet.HatchedAt())
	assert.Equal(t, StageBaby, pet.Stage())

	assert.False(t, pet.Hatch(testTime().Add(time.Hour)))
	assert.Equal(t, testTime(), *pet.HatchedAt())
}

func legacyUnhatchedPet() *Pet {
	return Restore(RestoreParams{
		ID: uuid.New(), UserID: uuid.New(), Stage: StageBaby, Level: 1,
		Satiety: 50, Happiness: 50, Energy: 100,
		LastDecayTime: testTime(), UpdatedAt: testTime(),
	})
}

func TestHatchedAtReturnsCopy(t *testing.T) {
	t.Parallel()

	pet := New(petWithParams(50, 50, 100).UserID(), testTime())

	got := pet.HatchedAt()
	require.NotNil(t, got)
	*got = got.Add(48 * time.Hour)

	assert.Equal(t, testTime(), *pet.HatchedAt())
}
