package domain_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hel1th/pet-economy/pet/domain"
)

func questDay() time.Time {
	return time.Date(2026, time.March, 5, 2, 0, 0, 0, time.UTC)
}

func questIDs(quests []domain.Quest) []string {
	ids := make([]string, 0, len(quests))
	for _, quest := range quests {
		ids = append(ids, quest.ID)
	}

	return ids
}

func TestDailyQuestsAreStableWithinTheDay(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	morning := questDay()
	evening := morning.Add(9 * time.Hour)

	assert.Equal(
		t,
		questIDs(domain.DailyQuests(userID, morning)),
		questIDs(domain.DailyQuests(userID, evening)),
		"quest list must not change during the day",
	)
}

func TestDailyQuestsChangeAcrossDaysOrUsers(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	today := questIDs(domain.DailyQuests(userID, questDay()))

	differentDay := false
	for offset := 1; offset <= 90; offset++ {
		next := questIDs(domain.DailyQuests(userID, questDay().AddDate(0, 0, offset)))
		if !assert.ObjectsAreEqual(today, next) {
			differentDay = true

			break
		}
	}
	assert.True(t, differentDay, "quest selection must vary across days")

	differentUser := false
	for range 20 {
		other := questIDs(domain.DailyQuests(uuid.New(), questDay()))
		if !assert.ObjectsAreEqual(today, other) {
			differentUser = true

			break
		}
	}
	assert.True(t, differentUser, "quest selection must vary across users")
}

func TestDailyQuestsReturnDistinctQuests(t *testing.T) {
	t.Parallel()

	quests := domain.DailyQuests(uuid.New(), questDay())
	require.Len(t, quests, domain.QuestsPerDay)

	seen := make(map[string]struct{}, len(quests))
	for _, quest := range quests {
		_, duplicate := seen[quest.ID]
		require.False(t, duplicate, "quest %s selected twice", quest.ID)
		seen[quest.ID] = struct{}{}

		assert.Positive(t, quest.Target)
		assert.Positive(t, quest.Reward)
		assert.NotEmpty(t, quest.Action)
	}
}

func TestEvaluateQuestsTracksProgressAndCompletion(t *testing.T) {
	t.Parallel()

	quests := []domain.Quest{
		{ID: "favorite_three", Action: domain.ActionFavorite, Target: 3, Reward: 5},
		{ID: "sell_one", Action: domain.ActionItemSold, Target: 1, Reward: 15},
	}

	progress := domain.EvaluateQuests(quests, map[domain.Action]int{
		domain.ActionFavorite: 2,
	})

	require.Len(t, progress, 2)

	assert.Equal(t, 2, progress[0].Current)
	assert.False(t, progress[0].Completed)

	assert.Equal(t, 0, progress[1].Current)
	assert.False(t, progress[1].Completed)
}

func TestEvaluateQuestsClampsProgressToTarget(t *testing.T) {
	t.Parallel()

	quests := []domain.Quest{
		{ID: "favorite_three", Action: domain.ActionFavorite, Target: 3, Reward: 5},
	}

	progress := domain.EvaluateQuests(quests, map[domain.Action]int{domain.ActionFavorite: 9})

	require.Len(t, progress, 1)
	assert.Equal(t, 3, progress[0].Current, "progress must never exceed the target")
	assert.True(t, progress[0].Completed)
}

func TestQuestCatalogUsesOnlySupportedActions(t *testing.T) {
	t.Parallel()

	supported := map[domain.Action]struct{}{
		domain.ActionFavorite:       {},
		domain.ActionItemViewed:     {},
		domain.ActionItemPublished:  {},
		domain.ActionItemUpdated:    {},
		domain.ActionItemImproved:   {},
		domain.ActionItemSold:       {},
		domain.ActionQualityListing: {},
	}

	for _, quest := range domain.QuestCatalog() {
		_, ok := supported[quest.Action]
		assert.True(t, ok, "quest %s uses unsupported action %s", quest.ID, quest.Action)
	}
}
