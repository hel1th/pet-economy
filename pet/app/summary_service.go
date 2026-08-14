package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/hel1th/pet-economy/pet/domain"
	"github.com/hel1th/pet-economy/shared/domainerr"
	"github.com/hel1th/pet-economy/shared/pagination"
)

type SummaryServiceDeps struct {
	Summaries SummaryRepository
	Collector *FactCollector
	Generator SummaryGenerator
	Tx        TxManager
	Clock     Clock
}

type SummaryService struct {
	summaries SummaryRepository
	collector *FactCollector
	generator SummaryGenerator
	tx        TxManager
	clock     Clock
}

func NewSummaryService(deps SummaryServiceDeps) *SummaryService {
	return &SummaryService{
		summaries: deps.Summaries, collector: deps.Collector,
		generator: deps.Generator, tx: deps.Tx, clock: deps.Clock,
	}
}

func (s *SummaryService) Today(ctx context.Context, userID uuid.UUID) (*domain.DailySummary, error) {
	date := domain.PreviousDay(s.clock.Now())

	existing, err := s.summaries.ByUserAndDate(ctx, userID, date)
	if err != nil && !errors.Is(err, domainerr.ErrNotFound) {
		return nil, err
	}
	if existing != nil {
		return existing, nil
	}

	return s.generate(ctx, userID, date)
}

func (s *SummaryService) generate(
	ctx context.Context,
	userID uuid.UUID,
	date time.Time,
) (*domain.DailySummary, error) {
	facts, err := s.collector.Collect(ctx, userID, date)
	if err != nil {
		return nil, err
	}

	message, source := s.message(ctx, facts)

	summary, err := domain.NewDailySummary(
		userID, facts, message, domain.BuildAdvice(facts), source, s.clock.Now(),
	)
	if err != nil {
		return nil, fmt.Errorf("build daily summary: %w", err)
	}

	return s.persist(ctx, userID, date, summary)
}

func (s *SummaryService) persist(
	ctx context.Context,
	userID uuid.UUID,
	date time.Time,
	summary *domain.DailySummary,
) (*domain.DailySummary, error) {
	var result *domain.DailySummary

	err := s.tx.WithTx(ctx, func(ctx context.Context) error {
		inserted, insertErr := s.summaries.Insert(ctx, summary)
		if insertErr != nil {
			return insertErr
		}
		if inserted {
			result = summary

			return nil
		}

		stored, readErr := s.summaries.ByUserAndDate(ctx, userID, date)
		if readErr != nil {
			return readErr
		}
		result = stored

		return nil
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (s *SummaryService) message(ctx context.Context, facts domain.DayFacts) (string, domain.SummarySource) {
	fallback := domain.BuildSummaryMessage(facts)
	if s.generator == nil {
		return fallback, domain.SummarySourceTemplate
	}

	generated, ok := s.generator.Generate(ctx, facts)
	if !ok || generated == "" {
		return fallback, domain.SummarySourceTemplate
	}

	return generated, domain.SummarySourceLLM
}

type SummaryHistoryQuery struct {
	UserID uuid.UUID
	Cursor pagination.Cursor
	Limit  int
}

type SummaryHistoryResult struct {
	Items      []*domain.DailySummary
	NextCursor string
}

func (s *SummaryService) History(
	ctx context.Context,
	q SummaryHistoryQuery,
) (SummaryHistoryResult, error) {
	limit := pagination.NormalizeLimit(q.Limit)

	filter := SummaryHistoryFilter{UserID: q.UserID, Limit: limit + 1}
	if !q.Cursor.IsZero() {
		filter.Before = q.Cursor.CreatedAt
	}

	rows, err := s.summaries.ListByUser(ctx, filter)
	if err != nil {
		return SummaryHistoryResult{}, fmt.Errorf("list summaries: %w", err)
	}

	page, next := pagination.Paginate(rows, limit, summaryCursorKey)

	return SummaryHistoryResult{Items: page, NextCursor: next}, nil
}

func summaryCursorKey(summary *domain.DailySummary) pagination.CursorKey {
	return pagination.CursorKey{CreatedAt: summary.Date(), ID: summary.ID()}
}
