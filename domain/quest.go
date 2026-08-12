package domain

import (
	"hash/fnv"
	"sort"
	"time"

	"github.com/google/uuid"
)

const QuestsPerDay = 3

type Quest struct {
	ID     string
	Action Action
	Target int
	Reward int
}

type QuestProgress struct {
	Quest     Quest
	Current   int
	Completed bool
}

var questCatalog = []Quest{
	{ID: "favorite_three", Action: ActionFavorite, Target: 3, Reward: 5},
	{ID: "view_five", Action: ActionItemViewed, Target: 5, Reward: 5},
	{ID: "publish_one", Action: ActionItemPublished, Target: 1, Reward: 10},
	{ID: "update_two", Action: ActionItemUpdated, Target: 2, Reward: 8},
	{ID: "improve_one", Action: ActionItemImproved, Target: 1, Reward: 12},
	{ID: "quality_one", Action: ActionQualityListing, Target: 1, Reward: 10},
	{ID: "sell_one", Action: ActionItemSold, Target: 1, Reward: 15},
}

func QuestCatalog() []Quest {
	catalog := make([]Quest, len(questCatalog))
	copy(catalog, questCatalog)

	return catalog
}

func DailyQuests(userID uuid.UUID, now time.Time) []Quest {
	indexes := make([]int, len(questCatalog))
	for i := range questCatalog {
		indexes[i] = i
	}

	seed := questSeed(userID, now)
	sort.SliceStable(indexes, func(a, b int) bool {
		left := questWeight(seed, questCatalog[indexes[a]].ID)
		right := questWeight(seed, questCatalog[indexes[b]].ID)
		if left == right {
			return indexes[a] < indexes[b]
		}

		return left < right
	})

	count := QuestsPerDay
	if count > len(indexes) {
		count = len(indexes)
	}

	chosen := indexes[:count]
	sort.Ints(chosen)

	quests := make([]Quest, 0, count)
	for _, index := range chosen {
		quests = append(quests, questCatalog[index])
	}

	return quests
}

func EvaluateQuests(quests []Quest, counts map[Action]int) []QuestProgress {
	progress := make([]QuestProgress, 0, len(quests))
	for _, quest := range quests {
		current := counts[quest.Action]
		if current > quest.Target {
			current = quest.Target
		}
		progress = append(progress, QuestProgress{
			Quest:     quest,
			Current:   current,
			Completed: current >= quest.Target,
		})
	}

	return progress
}

func (p *Pet) AwardQuestReward(amount int, now time.Time) Progress {
	if amount <= 0 {
		return Progress{}
	}

	return p.applyAward(amount, now)
}

func questSeed(userID uuid.UUID, now time.Time) uint32 {
	hash := fnv.New32a()
	_, _ = hash.Write(userID[:])
	_, _ = hash.Write([]byte(day(now).Format(time.DateOnly)))

	return hash.Sum32()
}

func questWeight(seed uint32, questID string) uint32 {
	hash := fnv.New32a()
	_, _ = hash.Write([]byte(questID))

	return hash.Sum32() ^ seed
}
