package app_test

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/hel1th/pet-economy/pet/app"
	"github.com/hel1th/pet-economy/pet/domain"
	"github.com/hel1th/pet-economy/shared/domainerr"
)

type stubSummaryPets struct {
	pet *domain.Pet
	err error
}

func (s *stubSummaryPets) ByUserID(context.Context, uuid.UUID) (*domain.Pet, error) {
	return s.pet, s.err
}

func (s *stubSummaryPets) ByUserIDForUpdate(context.Context, uuid.UUID) (*domain.Pet, error) {
	return s.pet, s.err
}

func (s *stubSummaryPets) Save(context.Context, *domain.Pet) error { return nil }

type stubActivity struct {
	aggregates []app.XPAggregate
	before     int
	rewards    []string
	badges     []string
	issues     []domain.ListingIssue
	err        error
}

func (s *stubActivity) XPByAction(
	context.Context, uuid.UUID, time.Time, time.Time,
) ([]app.XPAggregate, error) {
	return s.aggregates, s.err
}

func (s *stubActivity) XPTotalBefore(context.Context, uuid.UUID, time.Time) (int, error) {
	return s.before, nil
}

func (s *stubActivity) RewardsGranted(
	context.Context, uuid.UUID, time.Time, time.Time,
) ([]string, error) {
	return s.rewards, nil
}

func (s *stubActivity) BadgesEarned(
	context.Context, uuid.UUID, time.Time, time.Time,
) ([]string, error) {
	return s.badges, nil
}

func (s *stubActivity) ListingIssues(
	context.Context, uuid.UUID, int,
) ([]domain.ListingIssue, error) {
	return s.issues, nil
}

type stubRank struct {
	rank  int
	found bool
}

func (s stubRank) RankOf(context.Context, uuid.UUID) (rank int, found bool, err error) {
	return s.rank, s.found, nil
}

type stubSummaryRepo struct {
	stored  map[string]*domain.DailySummary
	inserts int
	history []*domain.DailySummary
}

func newStubSummaryRepo() *stubSummaryRepo {
	return &stubSummaryRepo{stored: make(map[string]*domain.DailySummary)}
}

func summaryKey(userID uuid.UUID, date time.Time) string {
	return userID.String() + "|" + date.Format(time.DateOnly)
}

func (s *stubSummaryRepo) ByUserAndDate(
	_ context.Context, userID uuid.UUID, date time.Time,
) (*domain.DailySummary, error) {
	summary, ok := s.stored[summaryKey(userID, date)]
	if !ok {
		return nil, domainerr.ErrNotFound
	}

	return summary, nil
}

func (s *stubSummaryRepo) Insert(_ context.Context, summary *domain.DailySummary) (bool, error) {
	key := summaryKey(summary.UserID(), summary.Date())
	if _, exists := s.stored[key]; exists {
		return false, nil
	}

	s.stored[key] = summary
	s.inserts++

	return true, nil
}

func (s *stubSummaryRepo) ListByUser(
	context.Context, app.SummaryHistoryFilter,
) ([]*domain.DailySummary, error) {
	return s.history, nil
}

type stubTx struct{}

func (stubTx) WithTx(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

type stubClock struct{ now time.Time }

func (c stubClock) Now() time.Time { return c.now }

type stubGenerator struct {
	text string
	ok   bool
}

func (s stubGenerator) Generate(context.Context, domain.DayFacts) (string, bool) {
	return s.text, s.ok
}

type failingFactsGenerator struct{}

func (failingFactsGenerator) GenerateSummary(context.Context, any) (string, error) {
	return "", errors.New("network unreachable")
}

type okFactsGenerator struct{ text string }

func (g okFactsGenerator) GenerateSummary(context.Context, any) (string, error) {
	return g.text, nil
}
