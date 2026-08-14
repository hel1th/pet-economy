package domain

import (
	"time"

	"github.com/google/uuid"
)

type Badge struct {
	id          string
	name        string
	description string
	iconURL     string
}

func NewBadge(id, name, description, iconURL string) Badge {
	return Badge{id: id, name: name, description: description, iconURL: iconURL}
}

func (b Badge) ID() string          { return b.id }
func (b Badge) Name() string        { return b.name }
func (b Badge) Description() string { return b.description }
func (b Badge) IconURL() string     { return b.iconURL }

type EarnedBadge struct {
	badge    Badge
	userID   uuid.UUID
	earnedAt time.Time
}

func NewEarnedBadge(badge Badge, userID uuid.UUID, earnedAt time.Time) EarnedBadge {
	return EarnedBadge{badge: badge, userID: userID, earnedAt: earnedAt}
}

func (e EarnedBadge) Badge() Badge        { return e.badge }
func (e EarnedBadge) UserID() uuid.UUID   { return e.userID }
func (e EarnedBadge) EarnedAt() time.Time { return e.earnedAt }
func (e EarnedBadge) ID() string          { return e.badge.id }
func (e EarnedBadge) Name() string        { return e.badge.name }
func (e EarnedBadge) Description() string { return e.badge.description }
func (e EarnedBadge) IconURL() string     { return e.badge.iconURL }

type BadgeRule struct {
	ID     string
	Action Action
	Target int
}

type BadgeStats struct {
	Level      int
	StreakDays int
	Actions    map[Action]int
	QuestsDone int
}

type BadgeProgress struct {
	ID      string
	Current int
	Target  int
	Earned  bool
}

const (
	badgeRaccoonFriend = "raccoon_friend"
	badgeChargedStreak = "charged_streak"
	badgeQuestRunner   = "quest_runner"
)

var badgeRules = []BadgeRule{
	{ID: "explorer", Action: ActionFavorite, Target: 5},
	{ID: "curious_eye", Action: ActionItemViewed, Target: 10},
	{ID: "first_listing", Action: ActionItemPublished, Target: 1},
	{ID: "nothing_hidden", Action: ActionItemImproved, Target: 3},
	{ID: "green_planet", Action: ActionItemSold, Target: 5},
}

const (
	raccoonFriendLevel = 10
	chargedStreakDays  = 14
	questRunnerPerDay  = QuestsPerDay
)

func EvaluateBadges(stats BadgeStats) []BadgeProgress {
	progress := make([]BadgeProgress, 0, len(badgeRules)+3)
	for _, rule := range badgeRules {
		progress = append(progress, newBadgeProgress(rule.ID, stats.Actions[rule.Action], rule.Target))
	}

	progress = append(progress,
		newBadgeProgress(badgeRaccoonFriend, stats.Level, raccoonFriendLevel),
		newBadgeProgress(badgeChargedStreak, stats.StreakDays, chargedStreakDays),
		newBadgeProgress(badgeQuestRunner, stats.QuestsDone, questRunnerPerDay),
	)

	return progress
}

func BadgesEarnedBy(stats BadgeStats) []string {
	earned := make([]string, 0)
	for _, item := range EvaluateBadges(stats) {
		if item.Earned {
			earned = append(earned, item.ID)
		}
	}

	return earned
}

func newBadgeProgress(id string, current, target int) BadgeProgress {
	if current > target {
		current = target
	}

	return BadgeProgress{ID: id, Current: current, Target: target, Earned: current >= target}
}
