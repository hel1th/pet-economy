package domain_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/hel1th/pet-economy/domain"
)

func TestActionCount(t *testing.T) {
	t.Parallel()

	facts := domain.DayFacts{
		Date: time.Now(),
		Actions: []domain.XPByAction{
			{Action: domain.ActionFavorite, Count: 3},
			{Action: domain.ActionItemViewed, Count: 5},
		},
	}

	assert.Equal(t, 3, facts.ActionCount(domain.ActionFavorite))
	assert.Equal(t, 5, facts.ActionCount(domain.ActionItemViewed))
	assert.Equal(t, 0, facts.ActionCount(domain.ActionItemPublished))
}
