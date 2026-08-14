package app

import (
	"context"

	"github.com/hel1th/pet-economy/pet/domain"
)

type factsGenerator interface {
	GenerateSummary(ctx context.Context, facts any) (string, error)
}

type LLMSummaryGenerator struct {
	inner factsGenerator
}

func NewLLMSummaryGenerator(inner factsGenerator) *LLMSummaryGenerator {
	return &LLMSummaryGenerator{inner: inner}
}

func (g *LLMSummaryGenerator) Generate(ctx context.Context, facts domain.DayFacts) (string, bool) {
	if g == nil || g.inner == nil {
		return "", false
	}

	text, err := g.inner.GenerateSummary(ctx, facts)
	if err != nil || text == "" {
		return "", false
	}

	return text, true
}
