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
)

type stubAccountGate struct {
	active bool
	err    error
	calls  int
}

func (g *stubAccountGate) IsActive(context.Context, uuid.UUID) (bool, error) {
	g.calls++

	return g.active, g.err
}

type subjectAwareJournal struct {
	*fakeJournal
	seenSubject bool
	existsErr   error
	countErr    error
}

func (j *subjectAwareJournal) ExistsBySubject(
	context.Context, uuid.UUID, domain.Action, uuid.UUID,
) (bool, error) {
	return j.seenSubject, j.existsErr
}

func (j *subjectAwareJournal) CountSince(
	ctx context.Context, userID uuid.UUID, action domain.Action, since time.Time,
) (int, error) {
	if j.countErr != nil {
		return 0, j.countErr
	}

	return j.fakeJournal.CountSince(ctx, userID, action, since)
}

func publishCommand(userID uuid.UUID) app.AwardCommand {
	return app.AwardCommand{
		UserID:    userID,
		Action:    domain.ActionItemPublished,
		SubjectID: uuid.New(),
		Apply: func(p *domain.Pet, a domain.LimitedAction) error {
			_, err := p.RewardItemPublished(a)

			return err
		},
	}
}

func TestAwardActionSkipsInactiveAccount(t *testing.T) {
	t.Parallel()

	gate := &stubAccountGate{active: false}
	service, journal, _ := newTestService(t)
	service = service.WithAccountGate(gate)

	pet, err := service.AwardAction(t.Context(), publishCommand(uuid.New()))

	require.ErrorIs(t, err, domainerr.ErrNotFound)
	assert.Nil(t, pet)
	assert.Equal(t, 1, gate.calls)
	assert.Empty(t, journal.events)
}

func TestAwardActionPropagatesAccountGateFailure(t *testing.T) {
	t.Parallel()

	gate := &stubAccountGate{err: errors.New("users service down")}
	service, _, _ := newTestService(t)
	service = service.WithAccountGate(gate)

	_, err := service.AwardAction(t.Context(), publishCommand(uuid.New()))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "check account state")
}

func TestAwardActionProceedsForActiveAccount(t *testing.T) {
	t.Parallel()

	gate := &stubAccountGate{active: true}
	service, journal, _ := newTestService(t)
	service = service.WithAccountGate(gate)
	userID := uuid.New()

	pet, err := service.AwardAction(t.Context(), publishCommand(userID))

	require.NoError(t, err)
	require.NotNil(t, pet)
	assert.Equal(t, userID, pet.UserID())
	assert.Len(t, journal.events, 1)
}

func TestAwardActionWithoutGateTreatsAccountAsActive(t *testing.T) {
	t.Parallel()

	service, journal, _ := newTestService(t)

	pet, err := service.AwardAction(t.Context(), publishCommand(uuid.New()))

	require.NoError(t, err)
	require.NotNil(t, pet)
	assert.Len(t, journal.events, 1)
}

func TestAwardActionPropagatesCountFailure(t *testing.T) {
	t.Parallel()

	journal := &subjectAwareJournal{fakeJournal: newFakeJournal(), countErr: errors.New("db timeout")}
	service := app.NewService(
		newFakeRepository(), journal, newFakeCache(), passthroughTx{}, fixedClock{now: testNow()},
	)

	_, err := service.AwardAction(t.Context(), publishCommand(uuid.New()))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "count rewarded actions")
}

func TestAwardActionPropagatesUniquenessFailure(t *testing.T) {
	t.Parallel()

	journal := &subjectAwareJournal{fakeJournal: newFakeJournal(), existsErr: errors.New("index missing")}
	service := app.NewService(
		newFakeRepository(), journal, newFakeCache(), passthroughTx{}, fixedClock{now: testNow()},
	)

	_, err := service.AwardAction(t.Context(), publishCommand(uuid.New()))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "check action uniqueness")
}

func TestAwardActionMarksRepeatedSubjectAsNotUnique(t *testing.T) {
	t.Parallel()

	journal := &subjectAwareJournal{fakeJournal: newFakeJournal(), seenSubject: true}
	service := app.NewService(
		newFakeRepository(), journal, newFakeCache(), passthroughTx{}, fixedClock{now: testNow()},
	)

	var observed domain.LimitedAction
	_, err := service.AwardAction(t.Context(), app.AwardCommand{
		UserID:    uuid.New(),
		Action:    domain.ActionItemPublished,
		SubjectID: uuid.New(),
		Apply: func(_ *domain.Pet, a domain.LimitedAction) error {
			observed = a

			return nil
		},
	})

	require.NoError(t, err)
	assert.False(t, observed.Unique)
}

func TestAwardActionSkipsJournalWhenNoXPGranted(t *testing.T) {
	t.Parallel()

	service, journal, _ := newTestService(t)

	pet, err := service.AwardAction(t.Context(), app.AwardCommand{
		UserID:    uuid.New(),
		Action:    domain.ActionItemPublished,
		SubjectID: uuid.New(),
		Apply:     func(*domain.Pet, domain.LimitedAction) error { return nil },
	})

	require.NoError(t, err)
	require.NotNil(t, pet)
	assert.Empty(t, journal.events)
}

func TestAwardActionPropagatesApplyFailure(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("business rule violated")
	service, journal, _ := newTestService(t)

	_, err := service.AwardAction(t.Context(), app.AwardCommand{
		UserID:    uuid.New(),
		Action:    domain.ActionItemPublished,
		SubjectID: uuid.New(),
		Apply:     func(*domain.Pet, domain.LimitedAction) error { return sentinel },
	})

	require.ErrorIs(t, err, sentinel)
	assert.Empty(t, journal.events)
}

func TestAwardActionPassesRewardedCountFromJournal(t *testing.T) {
	t.Parallel()

	service, _, _ := newTestService(t)
	userID := uuid.New()

	require.NotPanics(t, func() {
		_, err := service.AwardAction(t.Context(), publishCommand(userID))
		require.NoError(t, err)
	})

	var observed domain.LimitedAction
	_, err := service.AwardAction(t.Context(), app.AwardCommand{
		UserID:    userID,
		Action:    domain.ActionItemPublished,
		SubjectID: uuid.New(),
		Apply: func(_ *domain.Pet, a domain.LimitedAction) error {
			observed = a

			return nil
		},
	})

	require.NoError(t, err)
	assert.Equal(t, 1, observed.RewardedCount)
	assert.Equal(t, testNow(), observed.Now)
}
