package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var ErrInvalidSummary = errors.New("invalid daily summary data")

type SummarySource string

const (
	SummarySourceTemplate SummarySource = "template"
	SummarySourceLLM      SummarySource = "llm"
)

func (s SummarySource) Valid() bool {
	switch s {
	case SummarySourceTemplate, SummarySourceLLM:
		return true
	default:
		return false
	}
}

type AdviceAction string

const (
	AdviceAddPhoto       AdviceAction = "add_photo"
	AdviceExpandText     AdviceAction = "expand_description"
	AdviceRefreshListing AdviceAction = "refresh_listing"
	AdviceSetPrice       AdviceAction = "set_price"
	AdviceCheckIn        AdviceAction = "check_in"
)

type Advice struct {
	Text      string       `json:"text"`
	ItemID    *uuid.UUID   `json:"item_id,omitempty"`
	ItemTitle string       `json:"item_title,omitempty"`
	Action    AdviceAction `json:"action"`
}

type DailySummary struct {
	id          uuid.UUID
	userID      uuid.UUID
	date        time.Time
	facts       DayFacts
	message     string
	advice      *Advice
	generatedBy SummarySource
	createdAt   time.Time
}

type RestoreSummaryParams struct {
	ID          uuid.UUID
	UserID      uuid.UUID
	Date        time.Time
	Facts       DayFacts
	Message     string
	Advice      *Advice
	GeneratedBy SummarySource
	CreatedAt   time.Time
}

func NewDailySummary(
	userID uuid.UUID,
	facts DayFacts,
	message string,
	advice *Advice,
	source SummarySource,
	now time.Time,
) (*DailySummary, error) {
	if userID == uuid.Nil || message == "" || !source.Valid() {
		return nil, ErrInvalidSummary
	}

	return &DailySummary{
		id:          uuid.New(),
		userID:      userID,
		date:        DayStart(facts.Date),
		facts:       facts,
		message:     message,
		advice:      advice,
		generatedBy: source,
		createdAt:   now,
	}, nil
}

func RestoreDailySummary(p RestoreSummaryParams) *DailySummary {
	return &DailySummary{
		id: p.ID, userID: p.UserID, date: p.Date, facts: p.Facts, message: p.Message,
		advice: p.Advice, generatedBy: p.GeneratedBy, createdAt: p.CreatedAt,
	}
}

func (s *DailySummary) ID() uuid.UUID              { return s.id }
func (s *DailySummary) UserID() uuid.UUID          { return s.userID }
func (s *DailySummary) Date() time.Time            { return s.date }
func (s *DailySummary) Facts() DayFacts            { return s.facts }
func (s *DailySummary) Message() string            { return s.message }
func (s *DailySummary) Advice() *Advice            { return s.advice }
func (s *DailySummary) GeneratedBy() SummarySource { return s.generatedBy }
func (s *DailySummary) CreatedAt() time.Time       { return s.createdAt }

func LevelForXP(xp int) int {
	level := initialLevel
	for level < MaxLevel && xp >= levelThresholds[level+1] {
		level++
	}

	return level
}

func PreviousDay(now time.Time) time.Time {
	return DayStart(now).AddDate(0, 0, -1)
}

func DayEnd(dayStart time.Time) time.Time {
	return dayStart.AddDate(0, 0, 1)
}
