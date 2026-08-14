package app_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hel1th/pet-economy/pet/app"
	"github.com/hel1th/pet-economy/pet/domain"
	"github.com/hel1th/pet-economy/shared/events"
)

type failingSaveRepository struct {
	*fakeRepository
	err error
}

func (r *failingSaveRepository) Save(context.Context, *domain.Pet) error { return r.err }

func newRewardingSubscriber(t *testing.T, repo *fakeRewardRepository) (*app.Subscriber, *spyRewardNotifier) {
	t.Helper()

	pets := newFakeRepository()
	service := app.NewService(pets, newFakeJournal(), newFakeCache(), passthroughTx{}, fixedClock{now: testNow()})
	notifier := &spyRewardNotifier{}
	rewards := newRewardService(repo, pets, &stubSigner{}, notifier)

	return app.NewSubscriber(service, &spyNotifier{}, fixedClock{now: testNow()}).WithRewards(rewards), notifier
}

func TestWithRewardsReturnsSameSubscriber(t *testing.T) {
	t.Parallel()

	service := app.NewService(
		newFakeRepository(), newFakeJournal(), newFakeCache(), passthroughTx{}, fixedClock{now: testNow()})
	subscriber := app.NewSubscriber(service, &spyNotifier{}, fixedClock{now: testNow()})

	returned := subscriber.WithRewards(nil)

	assert.Same(t, subscriber, returned)
}

func TestSubscriberGrantsRewardsOnLevelUp(t *testing.T) {
	t.Parallel()

	repo := newFakeRewardRepository(levelReward("promo-l2", 2))
	subscriber, notifier := newRewardingSubscriber(t, repo)
	userID := uuid.New()

	err := subscriber.Dispatch(t.Context(), events.New(events.TypeItemSold, userID, uuid.New(), testNow()))

	require.NoError(t, err)
	assert.Equal(t, []string{"promo-l2"}, notifier.granted)
}

func TestSubscriberSkipsRewardsWithoutLevelUp(t *testing.T) {
	t.Parallel()

	repo := newFakeRewardRepository(levelReward("promo-l2", 2))
	subscriber, notifier := newRewardingSubscriber(t, repo)
	userID := uuid.New()

	err := subscriber.Dispatch(t.Context(), events.New(events.TypeFavoriteAdded, userID, uuid.New(), testNow()))

	require.NoError(t, err)
	assert.Empty(t, notifier.granted)
	assert.Zero(t, repo.grantCalls)
}

func TestSubscriberSwallowsRewardGrantFailure(t *testing.T) {
	t.Parallel()

	repo := newFakeRewardRepository(levelReward("promo-l2", 2))
	repo.catalogErr = errors.New("catalog unavailable")
	subscriber, notifier := newRewardingSubscriber(t, repo)

	err := subscriber.Dispatch(t.Context(),
		events.New(events.TypeItemSold, uuid.New(), uuid.New(), testNow()))

	require.NoError(t, err)
	assert.Empty(t, notifier.granted)
}

func TestSubscriberWithoutRewardsIgnoresLevelUp(t *testing.T) {
	t.Parallel()

	_, bus, notifier, _ := newSubscriberFixture(t)
	userID := uuid.New()

	assert.NotPanics(t, func() {
		bus.Publish(t.Context(), events.New(events.TypeItemSold, userID, uuid.New(), testNow()))
	})
	assert.NotEmpty(t, notifier.levelUps)
}

func TestDispatchIgnoresUnknownEventType(t *testing.T) {
	t.Parallel()

	subscriber, _, notifier, journal := newSubscriberFixture(t)

	err := subscriber.Dispatch(t.Context(),
		events.New(events.Type("item.archived"), uuid.New(), uuid.New(), testNow()))

	require.NoError(t, err)
	assert.Empty(t, notifier.updated)
	assert.Empty(t, journal.events)
}

func TestDispatchRoutesEveryRegisteredType(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		eventType events.Type
		action    domain.Action
	}{
		{name: "published", eventType: events.TypeItemPublished, action: domain.ActionItemPublished},
		{name: "sold", eventType: events.TypeItemSold, action: domain.ActionItemSold},
		{name: "favorite", eventType: events.TypeFavoriteAdded, action: domain.ActionFavorite},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			subscriber, _, _, journal := newSubscriberFixture(t)

			err := subscriber.Dispatch(t.Context(),
				events.New(tc.eventType, uuid.New(), uuid.New(), testNow()))

			require.NoError(t, err)
			require.Len(t, journal.events, 1)
			assert.Equal(t, tc.action, journal.events[0].Action)
		})
	}
}

func TestOnUserRegisteredPropagatesCreateFailure(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("pets table locked")
	repo := &failingSaveRepository{fakeRepository: newFakeRepository(), err: sentinel}
	service := app.NewService(repo, newFakeJournal(), newFakeCache(), passthroughTx{}, fixedClock{now: testNow()})
	notifier := &spyNotifier{}
	subscriber := app.NewSubscriber(service, notifier, fixedClock{now: testNow()})

	err := subscriber.Dispatch(t.Context(),
		events.New(events.TypeUserRegistered, uuid.New(), uuid.Nil, testNow()))

	require.ErrorIs(t, err, sentinel)
	assert.Empty(t, notifier.updated)
}

func TestSubscriberNotifiesLevelUpOnlyOnActualIncrease(t *testing.T) {
	t.Parallel()

	_, bus, notifier, _ := newSubscriberFixture(t)
	userID := uuid.New()

	bus.Publish(t.Context(), events.New(events.TypeFavoriteAdded, userID, uuid.New(), testNow()))
	bus.Publish(t.Context(), events.New(events.TypeFavoriteAdded, userID, uuid.New(), testNow()))

	assert.Len(t, notifier.xpGained, 2)
	assert.Empty(t, notifier.levelUps)
}
