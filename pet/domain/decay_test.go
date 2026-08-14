package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplyDecay(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		satiety       int
		happiness     int
		energy        int
		elapsed       time.Duration
		wantSatiety   int
		wantHappiness int
		wantEnergy    int
	}{
		{
			name: "zero interval keeps values", satiety: 70, happiness: 70, energy: 50,
			elapsed: 0, wantSatiety: 70, wantHappiness: 70, wantEnergy: 50,
		},
		{
			name: "negative interval is ignored", satiety: 70, happiness: 70, energy: 50,
			elapsed: -5 * time.Hour, wantSatiety: 70, wantHappiness: 70, wantEnergy: 50,
		},
		{
			name: "ten hours drains and restores", satiety: 100, happiness: 100, energy: 10,
			elapsed: 10 * time.Hour, wantSatiety: 80, wantHappiness: 85, wantEnergy: 60,
		},
		{
			name: "long idle clamps to bounds", satiety: 100, happiness: 100, energy: 0,
			elapsed: 100 * time.Hour, wantSatiety: 0, wantHappiness: 0, wantEnergy: 100,
		},
		{
			name: "already at floor stays at floor", satiety: 0, happiness: 0, energy: 100,
			elapsed: time.Hour, wantSatiety: 0, wantHappiness: 0, wantEnergy: 100,
		},
		{
			name: "fractional hour rounds", satiety: 70, happiness: 70, energy: 70,
			elapsed: 30 * time.Minute, wantSatiety: 69, wantHappiness: 69, wantEnergy: 73,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			pet := petWithParams(tt.satiety, tt.happiness, tt.energy)
			now := testTime().Add(tt.elapsed)
			pet.ApplyDecay(now)

			assert.Equal(t, tt.wantSatiety, pet.Satiety())
			assert.Equal(t, tt.wantHappiness, pet.Happiness())
			assert.Equal(t, tt.wantEnergy, pet.Energy())
		})
	}
}

func TestApplyDecayAdvancesLastDecayTime(t *testing.T) {
	t.Parallel()

	pet := New(uuid.New(), testTime())
	later := testTime().Add(3 * time.Hour)
	pet.ApplyDecay(later)

	assert.Equal(t, later, pet.LastDecayTime())

	pet.ApplyDecay(later)
	assert.Equal(t, 64, pet.Satiety())
}

func TestFeedAndCheerClamp(t *testing.T) {
	t.Parallel()

	pet := petWithParams(95, 95, 100)
	pet.Feed(30, testTime())
	pet.Cheer(30, testTime())

	assert.Equal(t, 100, pet.Satiety())
	assert.Equal(t, 100, pet.Happiness())
}

func TestFeedMealRaisesSatietyAndBumpsVersion(t *testing.T) {
	t.Parallel()

	pet := petWithParams(40, 50, 100)
	pet.FeedMeal(testTime())

	assert.Equal(t, 40+FeedSatietyGain, pet.Satiety())
	assert.Equal(t, 50, pet.Happiness())
	assert.Equal(t, int64(1), pet.InteractionVersion())
	assert.Equal(t, testTime(), pet.UpdatedAt())
}

func TestFeedMealClampsAtMaximum(t *testing.T) {
	t.Parallel()

	pet := petWithParams(95, 50, 100)
	pet.FeedMeal(testTime())
	pet.FeedMeal(testTime())

	assert.Equal(t, MaxParameterValue, pet.Satiety())
	assert.Equal(t, int64(2), pet.InteractionVersion())
}

func TestFeedMealLiftsPetOutOfHungryPenalty(t *testing.T) {
	t.Parallel()

	pet := petWithParams(20, 50, 100)
	require.Equal(t, StateSad, pet.State())

	pet.FeedMeal(testTime())

	assert.Equal(t, 35, pet.Satiety())
	assert.Equal(t, StateNeutral, pet.State())
}

func petWithParams(satiety, happiness, energy int) *Pet {
	return Restore(RestoreParams{
		ID: uuid.New(), UserID: uuid.New(), Stage: StageBaby, Level: 1,
		Satiety: satiety, Happiness: happiness, Energy: energy,
		LastDecayTime: testTime(), UpdatedAt: testTime(),
	})
}
