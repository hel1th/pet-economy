package domain

import (
	"time"
)

type XPByAction struct {
	Action Action `json:"action"`
	Count  int    `json:"count"`
	Amount int    `json:"amount"`
}

type PetSnapshot struct {
	Stage     Stage `json:"stage"`
	State     State `json:"state"`
	Satiety   int   `json:"satiety"`
	Happiness int   `json:"happiness"`
	Energy    int   `json:"energy"`
}

type StreakFacts struct {
	Days      int  `json:"days"`
	Broken    bool `json:"broken"`
	CheckedIn bool `json:"checked_in"`
}

type LevelFacts struct {
	Previous    int `json:"previous"`
	Current     int `json:"current"`
	XP          int `json:"xp"`
	NextLevelXP int `json:"next_level_xp"`
	XPToNext    int `json:"xp_to_next_level"`
}

type LeaderboardFacts struct {
	Rank     int  `json:"rank"`
	Previous int  `json:"previous_rank"`
	Known    bool `json:"known"`
}

type ListingIssueKind string

const (
	IssueNoPhoto   ListingIssueKind = "no_photo"
	IssueShortText ListingIssueKind = "short_description"
	IssueStale     ListingIssueKind = "stale"
	IssueNoPrice   ListingIssueKind = "no_price"
)

type ListingIssue struct {
	ItemID    string           `json:"item_id"`
	Title     string           `json:"title"`
	Kind      ListingIssueKind `json:"kind"`
	StaleDays int              `json:"stale_days,omitempty"`
}

type DayFacts struct {
	Date        time.Time        `json:"date"`
	PetName     string           `json:"pet_name"`
	TotalXP     int              `json:"total_xp"`
	Actions     []XPByAction     `json:"actions"`
	Level       LevelFacts       `json:"level"`
	LeveledUp   bool             `json:"leveled_up"`
	Rewards     []string         `json:"rewards"`
	Badges      []string         `json:"badges"`
	Pet         PetSnapshot      `json:"pet"`
	Streak      StreakFacts      `json:"streak"`
	Leaderboard LeaderboardFacts `json:"leaderboard"`
	Issues      []ListingIssue   `json:"issues"`
}

func (f DayFacts) IsEmpty() bool {
	return f.TotalXP == 0 && len(f.Actions) == 0 && !f.Streak.CheckedIn
}

func (f DayFacts) ActionCount(action Action) int {
	for _, entry := range f.Actions {
		if entry.Action == action {
			return entry.Count
		}
	}

	return 0
}

func (f DayFacts) TopIssue() (ListingIssue, bool) {
	priority := map[ListingIssueKind]int{
		IssueNoPhoto:   0,
		IssueNoPrice:   1,
		IssueShortText: 2,
		IssueStale:     3,
	}

	best := ListingIssue{}
	found := false

	for _, issue := range f.Issues {
		if !found || priority[issue.Kind] < priority[best.Kind] {
			best = issue
			found = true
		}
	}

	return best, found
}

func (f DayFacts) RankImproved() bool {
	return f.Leaderboard.Known && f.Leaderboard.Previous > 0 &&
		f.Leaderboard.Rank < f.Leaderboard.Previous
}

func (f DayFacts) RankDropped() bool {
	return f.Leaderboard.Known && f.Leaderboard.Previous > 0 &&
		f.Leaderboard.Rank > f.Leaderboard.Previous
}
