package app_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hel1th/pet-economy/pet/app"
	"github.com/hel1th/pet-economy/pet/domain"
)

func summaryNow() time.Time {
	return time.Date(2026, time.March, 10, 9, 0, 0, 0, time.UTC)
}

func newSummaryService(
	repo *stubSummaryRepo,
	generator app.SummaryGenerator,
) *app.SummaryService {
	checkIn := yesterday()

	return app.NewSummaryService(app.SummaryServiceDeps{
		Summaries: repo,
		Collector: app.NewFactCollector(app.FactCollectorDeps{
			Pets: &stubSummaryPets{pet: petWith(3, 14, 4, &checkIn)},
			Activity: &stubActivity{
				before:     10,
				aggregates: []app.XPAggregate{{Action: domain.ActionFavorite, Count: 4, Amount: 4}},
			},
			Rank: stubRank{rank: 5, found: true},
		}),
		Generator: generator,
		Tx:        stubTx{},
		Clock:     stubClock{now: summaryNow()},
	})
}

func TestTodayGeneratesSummaryWithTemplate(t *testing.T) {
	t.Parallel()

	repo := newStubSummaryRepo()
	service := newSummaryService(repo, nil)

	summary, err := service.Today(context.Background(), summaryUser())

	require.NoError(t, err)
	require.NotNil(t, summary)
	assert.Equal(t, domain.SummarySourceTemplate, summary.GeneratedBy())
	assert.NotEmpty(t, summary.Message())
	assert.NotNil(t, summary.Advice())
	assert.Equal(t, domain.PreviousDay(summaryNow()), summary.Date())
	assert.Equal(t, 1, repo.inserts)
}

func TestTodayIsIdempotent(t *testing.T) {
	t.Parallel()

	repo := newStubSummaryRepo()
	service := newSummaryService(repo, nil)

	first, err := service.Today(context.Background(), summaryUser())
	require.NoError(t, err)

	second, err := service.Today(context.Background(), summaryUser())
	require.NoError(t, err)

	assert.Equal(t, 1, repo.inserts)
	assert.Equal(t, first.ID(), second.ID())
	assert.Equal(t, first.Message(), second.Message())
}

func TestTodayUsesLLMWhenAvailable(t *testing.T) {
	t.Parallel()

	repo := newStubSummaryRepo()
	service := newSummaryService(repo, stubGenerator{text: "Мы отлично провели день!", ok: true})

	summary, err := service.Today(context.Background(), summaryUser())

	require.NoError(t, err)
	assert.Equal(t, domain.SummarySourceLLM, summary.GeneratedBy())
	assert.Equal(t, "Мы отлично провели день!", summary.Message())
}

func TestTodayFallsBackToTemplateOnGeneratorFailure(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		generator app.SummaryGenerator
	}{
		{name: "generator reports failure", generator: stubGenerator{ok: false}},
		{name: "generator returns empty text", generator: stubGenerator{text: "", ok: true}},
		{
			name:      "llm client errors out",
			generator: app.NewLLMSummaryGenerator(failingFactsGenerator{}),
		},
		{name: "no generator wired", generator: nil},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			repo := newStubSummaryRepo()
			service := newSummaryService(repo, tc.generator)

			summary, err := service.Today(context.Background(), summaryUser())

			require.NoError(t, err)
			assert.Equal(t, domain.SummarySourceTemplate, summary.GeneratedBy())
			assert.NotEmpty(t, summary.Message())
		})
	}
}

func TestLLMSummaryGeneratorSuccess(t *testing.T) {
	t.Parallel()

	generator := app.NewLLMSummaryGenerator(okFactsGenerator{text: "Привет!"})

	text, ok := generator.Generate(context.Background(), domain.DayFacts{Date: yesterday()})

	assert.True(t, ok)
	assert.Equal(t, "Привет!", text)
}

func TestHistoryPaginates(t *testing.T) {
	t.Parallel()

	repo := newStubSummaryRepo()
	for offset := range 4 {
		facts := domain.DayFacts{Date: yesterday().AddDate(0, 0, -offset), PetName: "Тоша"}
		summary, err := domain.NewDailySummary(
			summaryUser(), facts, "текст", nil, domain.SummarySourceTemplate, summaryNow(),
		)
		require.NoError(t, err)
		repo.history = append(repo.history, summary)
	}

	service := newSummaryService(repo, nil)

	result, err := service.History(context.Background(), app.SummaryHistoryQuery{
		UserID: summaryUser(), Limit: 3,
	})

	require.NoError(t, err)
	assert.Len(t, result.Items, 3)
	assert.NotEmpty(t, result.NextCursor)
}

func TestHistoryWithoutOverflowHasNoCursor(t *testing.T) {
	t.Parallel()

	repo := newStubSummaryRepo()
	facts := domain.DayFacts{Date: yesterday(), PetName: "Тоша"}
	summary, err := domain.NewDailySummary(
		summaryUser(), facts, "текст", nil, domain.SummarySourceTemplate, summaryNow(),
	)
	require.NoError(t, err)
	repo.history = []*domain.DailySummary{summary}

	service := newSummaryService(repo, nil)

	result, err := service.History(context.Background(), app.SummaryHistoryQuery{
		UserID: summaryUser(), Limit: 10,
	})

	require.NoError(t, err)
	assert.Len(t, result.Items, 1)
	assert.Empty(t, result.NextCursor)
}
