package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPet(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.August, 3, 12, 0, 0, 0, time.UTC)
	pet := New(uuid.New(), now)

	assert.Equal(t, StageBaby, pet.Stage())
	assert.Equal(t, 1, pet.Level())
	assert.Equal(t, 5, pet.NextLevelXP())
	assert.Equal(t, 70, pet.Satiety())
	assert.Equal(t, 70, pet.Happiness())
	assert.Equal(t, now, pet.LastDecayTime())
	assert.True(t, pet.IsHatched())
	require.NotNil(t, pet.HatchedAt())
	assert.Equal(t, now, *pet.HatchedAt())
}

func TestPetRaisesHappinessAndClampsIt(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.August, 3, 12, 0, 0, 0, time.UTC)
	pet := New(uuid.New(), now)

	for range 10 {
		pet.Stroke(now.Add(time.Minute))
	}

	require.Equal(t, 100, pet.Happiness())
	assert.Equal(t, now.Add(time.Minute), pet.UpdatedAt())
}
