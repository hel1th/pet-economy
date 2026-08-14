package app_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hel1th/pet-economy/pet/app"
	"github.com/hel1th/pet-economy/pet/domain"
	"github.com/hel1th/pet-economy/shared/domainerr"
	"github.com/hel1th/pet-economy/shared/pagination"
)

type erroringSummaryRepo struct {
	*stubSummaryRepo
	readErr   error
	insertErr error
	listErr   error
	reads     int
}

func (r *erroringSummaryRepo) ByUserAndDate(
	ctx context.Context, userID uuid.UUID, date time.Time,
) (*domain.DailySummary, error) {
	r.reads++
	if r.readErr != nil {
		return nil, r.readErr
	}

	return r.stubSummaryRepo.ByUserAndDate(ctx, userID, date)
}

func (r *erroringSummaryRepo) Insert(ctx context.Context, summary *domain.DailySummary) (bool, error) {
	if r.insertErr != nil {
		return false, r.insertErr
	}

	return r.stubSummaryRepo.Insert(ctx, summary)
}

func (r *erroringSummaryRepo) ListByUser(
	ctx context.Context, filter app.SummaryHistoryFilter,
) ([]*domain.DailySummary, error) {
	if r.listErr != nil {
		return nil, r.listErr
	}

	return r.stubSummaryRepo.ListByUser(ctx, filter)
}

type racingSummaryRepo struct {
	*stubSummaryRepo
	stored *domain.DailySummary
}

func (r *racingSummaryRepo) Insert(context.Context, *domain.DailySummary) (bool, error) {
	return false, nil
}

func (r *racingSummaryRepo) ByUserAndDate(
	context.Context, uuid.UUID, time.Time,
) (*domain.DailySummary, error) {
	if r.stored == nil {
		return nil, domainerr.ErrNotFound
	}

	return r.stored, nil
}

func TestTodayPropagatesUnexpectedReadFailure(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("summaries table missing")
	repo := &erroringSummaryRepo{stubSummaryRepo: newStubSummaryRepo(), readErr: sentinel}
	service := app.NewSummaryService(app.SummaryServiceDeps{
		Summaries: repo,
		Collector: app.NewFactCollector(app.FactCollectorDeps{
			Pets:     &stubSummaryPets{pet: petWith(1, 0, 0, nil)},
			Activity: &stubActivity{},
		}),
		Tx:    stubTx{},
		Clock: stubClock{now: summaryNow()},
	})

	summary, err := service.Today(t.Context(), summaryUser())

	require.ErrorIs(t, err, sentinel)
	assert.Nil(t, summary)
}

func TestTodayPropagatesCollectorFailure(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("xp aggregation failed")
	service := app.NewSummaryService(app.SummaryServiceDeps{
		Summaries: newStubSummaryRepo(),
		Collector: app.NewFactCollector(app.FactCollectorDeps{
			Pets:     &stubSummaryPets{pet: petWith(1, 0, 0, nil)},
			Activity: &stubActivity{err: sentinel},
		}),
		Tx:    stubTx{},
		Clock: stubClock{now: summaryNow()},
	})

	summary, err := service.Today(t.Context(), summaryUser())

	require.ErrorIs(t, err, sentinel)
	assert.Nil(t, summary)
}

func TestTodayPropagatesPetLoadFailure(t *testing.T) {
	t.Parallel()

	service := app.NewSummaryService(app.SummaryServiceDeps{
		Summaries: newStubSummaryRepo(),
		Collector: app.NewFactCollector(app.FactCollectorDeps{
			Pets:     &stubSummaryPets{err: domainerr.ErrNotFound},
			Activity: &stubActivity{},
		}),
		Tx:    stubTx{},
		Clock: stubClock{now: summaryNow()},
	})

	summary, err := service.Today(t.Context(), summaryUser())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "load pet for summary")
	assert.Nil(t, summary)
}

func TestTodayPropagatesInsertFailure(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("unique violation escalated")
	repo := &erroringSummaryRepo{stubSummaryRepo: newStubSummaryRepo(), insertErr: sentinel}
	service := app.NewSummaryService(app.SummaryServiceDeps{
		Summaries: repo,
		Collector: app.NewFactCollector(app.FactCollectorDeps{
			Pets:     &stubSummaryPets{pet: petWith(1, 0, 0, nil)},
			Activity: &stubActivity{},
		}),
		Tx:    stubTx{},
		Clock: stubClock{now: summaryNow()},
	})

	summary, err := service.Today(t.Context(), summaryUser())

	require.ErrorIs(t, err, sentinel)
	assert.Nil(t, summary)
}

func TestPersistReturnsConcurrentlyStoredSummary(t *testing.T) {
	t.Parallel()

	facts := domain.DayFacts{Date: domain.PreviousDay(summaryNow()), PetName: "Tosha"}
	winner, err := domain.NewDailySummary(
		summaryUser(), facts, "already stored", nil, domain.SummarySourceTemplate, summaryNow(),
	)
	require.NoError(t, err)

	repo := &racingSummaryRepo{stubSummaryRepo: newStubSummaryRepo()}
	service := app.NewSummaryService(app.SummaryServiceDeps{
		Summaries: repo,
		Collector: app.NewFactCollector(app.FactCollectorDeps{
			Pets:     &stubSummaryPets{pet: petWith(1, 0, 0, nil)},
			Activity: &stubActivity{},
		}),
		Tx:    stubTx{},
		Clock: stubClock{now: summaryNow()},
	})

	repo.stored = winner

	summary, err := service.Today(t.Context(), summaryUser())

	require.NoError(t, err)
	require.NotNil(t, summary)
	assert.Equal(t, winner.ID(), summary.ID())
	assert.Equal(t, "already stored", summary.Message())
}

func TestHistoryWrapsRepositoryFailure(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("index scan aborted")
	repo := &erroringSummaryRepo{stubSummaryRepo: newStubSummaryRepo(), listErr: sentinel}
	service := app.NewSummaryService(app.SummaryServiceDeps{
		Summaries: repo,
		Tx:        stubTx{},
		Clock:     stubClock{now: summaryNow()},
	})

	result, err := service.History(t.Context(), app.SummaryHistoryQuery{UserID: summaryUser()})

	require.ErrorIs(t, err, sentinel)
	assert.Contains(t, err.Error(), "list summaries")
	assert.Empty(t, result.Items)
}

func TestHistoryAppliesCursorAsBeforeFilter(t *testing.T) {
	t.Parallel()

	repo := newStubSummaryRepo()
	facts := domain.DayFacts{Date: yesterday(), PetName: "Tosha"}
	summary, err := domain.NewDailySummary(
		summaryUser(), facts, "text", nil, domain.SummarySourceTemplate, summaryNow(),
	)
	require.NoError(t, err)
	repo.history = []*domain.DailySummary{summary}

	service := newSummaryService(repo, nil)

	result, err := service.History(t.Context(), app.SummaryHistoryQuery{
		UserID: summaryUser(),
		Limit:  5,
		Cursor: pagination.Cursor{CreatedAt: summaryNow(), ID: uuid.New()},
	})

	require.NoError(t, err)
	assert.Len(t, result.Items, 1)
}

func TestLLMSummaryGeneratorHandlesMissingInner(t *testing.T) {
	t.Parallel()

	var nilGenerator *app.LLMSummaryGenerator

	text, ok := nilGenerator.Generate(context.Background(), domain.DayFacts{})
	assert.False(t, ok)
	assert.Empty(t, text)

	text, ok = app.NewLLMSummaryGenerator(nil).Generate(context.Background(), domain.DayFacts{})
	assert.False(t, ok)
	assert.Empty(t, text)
}

func TestLLMSummaryGeneratorRejectsEmptyText(t *testing.T) {
	t.Parallel()

	text, ok := app.NewLLMSummaryGenerator(okFactsGenerator{text: ""}).
		Generate(context.Background(), domain.DayFacts{})

	assert.False(t, ok)
	assert.Empty(t, text)
}
