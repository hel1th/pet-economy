package clock_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/hel1th/pet-economy/shared/clock"
)

func TestClockReturnsUTC(t *testing.T) {
	t.Parallel()

	before := time.Now().UTC()
	now := clock.New().Now()
	after := time.Now().UTC()

	assert.Equal(t, time.UTC, now.Location())
	assert.False(t, now.Before(before))
	assert.False(t, now.After(after))
}
