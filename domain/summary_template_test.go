package domain_test

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hel1th/pet-economy/domain"
)

func summaryDay() time.Time {
	return domain.DayStart(time.Date(2026, time.March, 10, 12, 0, 0, 0, time.UTC))
}

func TestBuildSummaryMessageBranches(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		facts    domain.DayFacts
		contains []string
	}{
		{
			name:     "empty day invites to come back",
			facts:    domain.DayFacts{Date: summaryDay(), PetName: "Тоша"},
			contains: []string{"завтра"},
		},
		{
			name: "hungry empty day mentions state",
			facts: domain.DayFacts{
				Date: summaryDay(), PetName: "Тоша",
				Pet: domain.PetSnapshot{Satiety: 10},
			},
			contains: []string{"завтра"},
		},
		{
			name: "empty day with streak mentions state",
			facts: domain.DayFacts{
				Date: summaryDay(), PetName: "Тоша",
				Pet: domain.PetSnapshot{Satiety: 90},
				Streak: domain.StreakFacts{Days: 3},
			},
			contains: []string{"Серия", "завтра"},
		},
		{
			name: "active day reports xp",
			facts: domain.DayFacts{
				Date: summaryDay(), PetName: "Тоша", TotalXP: 12,
				Actions: []domain.XPByAction{
					{Action: domain.ActionQualityListing, Count: 2, Amount: 8},
				},
				Level:  domain.LevelFacts{Previous: 2, Current: 2},
				Streak: domain.StreakFacts{Days: 3, CheckedIn: true},
			},
			contains: []string{"12 XP", "объявлен", "3 дн"},
		},
		{
			name: "level up is announced",
			facts: domain.DayFacts{
				Date: summaryDay(), PetName: "Тоша", TotalXP: 30,
				Actions:   []domain.XPByAction{{Action: domain.ActionFavorite, Count: 4, Amount: 4}},
				Level:     domain.LevelFacts{Previous: 2, Current: 4},
				LeveledUp: true,
				Streak:    domain.StreakFacts{Days: 1, CheckedIn: true},
			},
			contains: []string{"4 уровня", "30 XP"},
		},
		{
			name: "broken streak is softened",
			facts: domain.DayFacts{
				Date: summaryDay(), PetName: "Тоша", TotalXP: 5,
				Actions: []domain.XPByAction{{Action: domain.ActionFavorite, Count: 2, Amount: 2}},
				Streak:  domain.StreakFacts{Days: 1, Broken: true},
			},
			contains: []string{"ери"},
		},
		{
			name: "leaderboard improvement is mentioned",
			facts: domain.DayFacts{
				Date: summaryDay(), PetName: "Тоша", TotalXP: 9,
				Actions:     []domain.XPByAction{{Action: domain.ActionItemViewed, Count: 3, Amount: 9}},
				Leaderboard: domain.LeaderboardFacts{Rank: 4, Previous: 9, Known: true},
			},
			contains: []string{"4 место"},
		},
		{
			name: "known rank without history is still shown",
			facts: domain.DayFacts{
				Date: summaryDay(), PetName: "Тоша", TotalXP: 3,
				Actions:     []domain.XPByAction{{Action: domain.ActionItemPublished, Count: 1, Amount: 3}},
				Leaderboard: domain.LeaderboardFacts{Rank: 12, Known: true},
			},
			contains: []string{"12 место"},
		},
		{
			name: "checked in without xp still yields a message",
			facts: domain.DayFacts{
				Date: summaryDay(), PetName: "Тоша",
				Streak: domain.StreakFacts{Days: 2, CheckedIn: true},
			},
			contains: []string{"2 дн"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			message := domain.BuildSummaryMessage(tc.facts)

			require.NotEmpty(t, message)
			assert.LessOrEqual(t, strings.Count(message, "."), 4)
			for _, fragment := range tc.contains {
				assert.Contains(t, message, fragment)
			}
		})
	}
}

func TestBuildSummaryMessageIsDeterministicPerDay(t *testing.T) {
	t.Parallel()

	facts := domain.DayFacts{Date: summaryDay(), PetName: "Тоша"}

	first := domain.BuildSummaryMessage(facts)
	second := domain.BuildSummaryMessage(facts)

	assert.Equal(t, first, second)
}

func TestBuildSummaryMessageVariesAcrossDays(t *testing.T) {
	t.Parallel()

	seen := make(map[string]struct{})
	for offset := range 10 {
		facts := domain.DayFacts{
			Date: summaryDay().AddDate(0, 0, offset), PetName: "Тоша",
		}
		seen[domain.BuildSummaryMessage(facts)] = struct{}{}
	}

	assert.Greater(t, len(seen), 1)
}

func TestBuildAdvice(t *testing.T) {
	t.Parallel()

	itemID := uuid.New()

	tests := []struct {
		name       string
		facts      domain.DayFacts
		wantAction domain.AdviceAction
		wantItem   bool
	}{
		{
			name:       "no issues yields check-in advice",
			facts:      domain.DayFacts{Date: summaryDay()},
			wantAction: domain.AdviceCheckIn,
		},
		{
			name: "missing photo wins over stale",
			facts: domain.DayFacts{
				Date: summaryDay(),
				Issues: []domain.ListingIssue{
					{ItemID: uuid.New(), Title: "Стул", Kind: domain.IssueStale, StaleDays: 30},
					{ItemID: itemID, Title: "iPhone 12", Kind: domain.IssueNoPhoto},
				},
			},
			wantAction: domain.AdviceAddPhoto,
			wantItem:   true,
		},
		{
			name: "short description",
			facts: domain.DayFacts{
				Date:   summaryDay(),
				Issues: []domain.ListingIssue{{ItemID: itemID, Title: "Стол", Kind: domain.IssueShortText}},
			},
			wantAction: domain.AdviceExpandText,
			wantItem:   true,
		},
		{
			name: "missing price",
			facts: domain.DayFacts{
				Date:   summaryDay(),
				Issues: []domain.ListingIssue{{ItemID: itemID, Title: "Диван", Kind: domain.IssueNoPrice}},
			},
			wantAction: domain.AdviceSetPrice,
			wantItem:   true,
		},
		{
			name: "stale listing",
			facts: domain.DayFacts{
				Date: summaryDay(),
				Issues: []domain.ListingIssue{
					{ItemID: itemID, Title: "Велосипед", Kind: domain.IssueStale, StaleDays: 22},
				},
			},
			wantAction: domain.AdviceRefreshListing,
			wantItem:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			advice := domain.BuildAdvice(tc.facts)

			require.NotNil(t, advice)
			assert.Equal(t, tc.wantAction, advice.Action)
			assert.NotEmpty(t, advice.Text)

			if tc.wantItem {
				require.NotNil(t, advice.ItemID)
				assert.Equal(t, itemID, *advice.ItemID)

				return
			}
			assert.Nil(t, advice.ItemID)
		})
	}
}
