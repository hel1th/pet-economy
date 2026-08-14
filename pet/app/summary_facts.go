package app

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/hel1th/pet-economy/pet/domain"
)

const maxListingIssues = 5

type FactCollectorDeps struct {
	Pets     domain.Repository
	Activity DayActivityReadModel
	Rank     RankReadModel
}

type FactCollector struct {
	deps FactCollectorDeps
}

func NewFactCollector(deps FactCollectorDeps) *FactCollector {
	return &FactCollector{deps: deps}
}

func (c *FactCollector) Collect(
	ctx context.Context,
	userID uuid.UUID,
	dayStart time.Time,
) (domain.DayFacts, error) {
	pet, err := c.deps.Pets.ByUserID(ctx, userID)
	if err != nil {
		return domain.DayFacts{}, fmt.Errorf("load pet for summary: %w", err)
	}

	facts := domain.DayFacts{Date: dayStart, PetName: pet.Name(), Pet: snapshotOf(pet)}
	dayEnd := domain.DayEnd(dayStart)

	if err := c.fillActivity(ctx, userID, dayStart, dayEnd, &facts); err != nil {
		return domain.DayFacts{}, err
	}

	c.fillLevels(ctx, userID, pet, dayStart, &facts)
	c.fillStreak(pet, dayStart, &facts)
	c.fillLeaderboard(ctx, userID, &facts)
	c.fillIssues(ctx, userID, &facts)

	return facts, nil
}

func (c *FactCollector) fillActivity(
	ctx context.Context,
	userID uuid.UUID,
	from, to time.Time,
	facts *domain.DayFacts,
) error {
	aggregates, err := c.deps.Activity.XPByAction(ctx, userID, from, to)
	if err != nil {
		return fmt.Errorf("aggregate xp events: %w", err)
	}

	actions := make([]domain.XPByAction, 0, len(aggregates))
	total := 0

	for _, aggregate := range aggregates {
		total += aggregate.Amount
		actions = append(actions, domain.XPByAction{
			Action: aggregate.Action, Count: aggregate.Count, Amount: aggregate.Amount,
		})
	}

	facts.Actions = actions
	facts.TotalXP = total

	rewards, err := c.deps.Activity.RewardsGranted(ctx, userID, from, to)
	if err != nil {
		return fmt.Errorf("collect granted rewards: %w", err)
	}
	facts.Rewards = rewards

	badges, err := c.deps.Activity.BadgesEarned(ctx, userID, from, to)
	if err != nil {
		return fmt.Errorf("collect earned badges: %w", err)
	}
	facts.Badges = badges

	return nil
}

func (c *FactCollector) fillLevels(
	ctx context.Context,
	userID uuid.UUID,
	pet *domain.Pet,
	dayStart time.Time,
	facts *domain.DayFacts,
) {
	toNext := pet.NextLevelXP() - pet.XP()
	if toNext < 0 || pet.IsMaxLevel() {
		toNext = 0
	}

	facts.Level = domain.LevelFacts{
		Previous: pet.Level(), Current: pet.Level(), XP: pet.XP(),
		NextLevelXP: pet.NextLevelXP(), XPToNext: toNext,
	}

	before, err := c.deps.Activity.XPTotalBefore(ctx, userID, dayStart)
	if err != nil {
		return
	}

	facts.Level.Previous = domain.LevelForXP(before)
	facts.LeveledUp = facts.Level.Current > facts.Level.Previous
}

func (c *FactCollector) fillStreak(pet *domain.Pet, dayStart time.Time, facts *domain.DayFacts) {
	facts.Streak = domain.StreakFacts{Days: pet.StreakDays()}

	last := pet.LastCheckInDate()
	if last == nil {
		facts.Streak.Broken = pet.StreakDays() == 0

		return
	}

	lastDay := domain.DayStart(*last)
	facts.Streak.CheckedIn = !lastDay.Before(dayStart) && lastDay.Before(domain.DayEnd(dayStart))
	facts.Streak.Broken = lastDay.Before(dayStart)
}

func (c *FactCollector) fillLeaderboard(ctx context.Context, userID uuid.UUID, facts *domain.DayFacts) {
	if c.deps.Rank == nil {
		return
	}

	rank, found, err := c.deps.Rank.RankOf(ctx, userID)
	if err != nil || !found {
		return
	}

	facts.Leaderboard = domain.LeaderboardFacts{Rank: rank, Known: true}
}

func (c *FactCollector) fillIssues(ctx context.Context, userID uuid.UUID, facts *domain.DayFacts) {
	issues, err := c.deps.Activity.ListingIssues(ctx, userID, maxListingIssues)
	if err != nil {
		return
	}

	facts.Issues = issues
}

func snapshotOf(pet *domain.Pet) domain.PetSnapshot {
	return domain.PetSnapshot{
		Stage: pet.Stage(), State: pet.State(), Satiety: pet.Satiety(),
		Happiness: pet.Happiness(), Energy: pet.Energy(),
	}
}
