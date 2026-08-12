package domain_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hel1th/pet-economy/domain"
)

func TestSummarySourceValid(t *testing.T) {
	t.Parallel()
	assert.True(t, domain.SummarySourceTemplate.Valid())
	assert.True(t, domain.SummarySourceLLM.Valid())
	assert.False(t, domain.SummarySource("invalid").Valid())
}

func TestNewDailySummary(t *testing.T) {
	t.Parallel()
	now := time.Now()
	facts := domain.DayFacts{Date: now, PetName: "Test"}
	
	_, err := domain.NewDailySummary(uuid.Nil, facts, "msg", nil, domain.SummarySourceTemplate, now)
	require.ErrorIs(t, err, domain.ErrInvalidSummary)
	
	userID := uuid.New()
	summary, err := domain.NewDailySummary(userID, facts, "msg", nil, domain.SummarySourceTemplate, now)
	require.NoError(t, err)
	
	assert.NotEqual(t, uuid.Nil, summary.ID())
	assert.Equal(t, userID, summary.UserID())
	assert.Equal(t, "msg", summary.Message())
	assert.Equal(t, domain.SummarySourceTemplate, summary.GeneratedBy())
	assert.Equal(t, now, summary.CreatedAt())
	assert.Equal(t, facts, summary.Facts())
	assert.Nil(t, summary.Advice())
	assert.NotZero(t, summary.Date())
}

func TestRestoreDailySummary(t *testing.T) {
	t.Parallel()
	id := uuid.New()
	userID := uuid.New()
	now := time.Now()
	facts := domain.DayFacts{Date: now}
	
	params := domain.RestoreSummaryParams{
		ID: id, UserID: userID, Date: now, Facts: facts,
		Message: "msg", Advice: nil, GeneratedBy: domain.SummarySourceLLM,
		CreatedAt: now,
	}
	
	summary := domain.RestoreDailySummary(params)
	assert.Equal(t, id, summary.ID())
	assert.Equal(t, userID, summary.UserID())
	assert.Equal(t, now, summary.Date())
}

func TestLevelForXP(t *testing.T) {
	t.Parallel()
	assert.Equal(t, 1, domain.LevelForXP(0))
	assert.Equal(t, 2, domain.LevelForXP(5))
	assert.Equal(t, 15, domain.LevelForXP(10000))
}

func TestPreviousDayEndDay(t *testing.T) {
	t.Parallel()
	now := time.Now()
	
	prev := domain.PreviousDay(now)
	assert.True(t, prev.Before(now))
	
	end := domain.DayEnd(now)
	assert.True(t, end.After(now))
}
