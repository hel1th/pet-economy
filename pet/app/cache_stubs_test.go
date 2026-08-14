package app_test

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"github.com/hel1th/pet-economy/pet/domain"
)

type brokenCache struct {
	getErr    error
	setErr    error
	deleteErr error
	deletes   int
	sets      int
}

func (c *brokenCache) Get(context.Context, uuid.UUID) (*domain.Pet, error) {
	return nil, c.getErr
}

func (c *brokenCache) Set(context.Context, *domain.Pet) error {
	c.sets++

	return c.setErr
}

func (c *brokenCache) Delete(context.Context, uuid.UUID) error {
	c.deletes++

	return c.deleteErr
}

type failingLoadRepository struct {
	*fakeRepository
	err error
}

func (r *failingLoadRepository) ByUserIDForUpdate(context.Context, uuid.UUID) (*domain.Pet, error) {
	return nil, r.err
}

type failingTx struct{ err error }

func (t failingTx) WithTx(context.Context, func(context.Context) error) error { return t.err }

func qualityDescription() string {
	return strings.Repeat("a", 240)
}
