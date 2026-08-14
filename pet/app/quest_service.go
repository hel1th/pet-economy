package app

import (
	"context"
	"crypto/sha256"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/hel1th/pet-economy/pet/domain"
	"github.com/hel1th/pet-economy/shared/domainerr"
)

type QuestView struct {
	Quest     domain.Quest
	Current   int
	Completed bool
	Claimed   bool
}

type QuestService struct {
	activity DayActivityReadModel
	journal  XPJournal
	pets     domain.Repository
	tx       TxManager
	clock    Clock
	notifier PetNotifier
}

type QuestServiceDeps struct {
	Activity DayActivityReadModel
	Journal  XPJournal
	Pets     domain.Repository
	Tx       TxManager
	Clock    Clock
	Notifier PetNotifier
}

func NewQuestService(deps QuestServiceDeps) *QuestService {
	return &QuestService{
		activity: deps.Activity, journal: deps.Journal, pets: deps.Pets,
		tx: deps.Tx, clock: deps.Clock, notifier: deps.Notifier,
	}
}

func (s *QuestService) Today(ctx context.Context, userID uuid.UUID) ([]QuestView, error) {
	now := s.clock.Now()
	quests := domain.DailyQuests(userID, now)

	counts, err := s.actionCounts(ctx, userID, now)
	if err != nil {
		return nil, err
	}

	views := make([]QuestView, 0, len(quests))
	for _, progress := range domain.EvaluateQuests(quests, counts) {
		views = append(views, QuestView{
			Quest:     progress.Quest,
			Current:   progress.Current,
			Completed: progress.Completed,
			Claimed:   counts[questRewardAction(progress.Quest.ID)] > 0,
		})
	}

	return views, nil
}

func (s *QuestService) ClaimCompleted(ctx context.Context, userID uuid.UUID) ([]QuestView, error) {
	views, err := s.Today(ctx, userID)
	if err != nil {
		return nil, err
	}

	now := s.clock.Now()
	for i := range views {
		if !views[i].Completed || views[i].Claimed {
			continue
		}
		if err := s.grant(ctx, userID, views[i].Quest, now); err != nil {
			return nil, err
		}
		views[i].Claimed = true
	}

	return views, nil
}

func (s *QuestService) grant(
	ctx context.Context,
	userID uuid.UUID,
	quest domain.Quest,
	now time.Time,
) error {
	subject := QuestSubjectID(userID, quest.ID, now)

	return s.tx.WithTx(ctx, func(ctx context.Context) error {
		pet, err := s.pets.ByUserID(ctx, userID)
		if err != nil {
			if errors.Is(err, domainerr.ErrNotFound) {
				return nil
			}

			return err
		}

		event := domain.NewXPEvent(userID, questRewardAction(quest.ID), &subject, quest.Reward, now)
		if err := s.journal.Append(ctx, event); err != nil {
			if errors.Is(err, domain.ErrDuplicateAction) {
				return nil
			}

			return err
		}

		progress := pet.AwardQuestReward(quest.Reward, now)

		if err := s.pets.Save(ctx, pet); err != nil {
			return err
		}

		s.notifyReward(userID, pet, progress, quest)

		return nil
	})
}

func (s *QuestService) notifyReward(
	userID uuid.UUID,
	pet *domain.Pet,
	progress domain.Progress,
	quest domain.Quest,
) {
	if s.notifier == nil {
		return
	}

	s.notifier.PetUpdated(userID, pet)

	if progress.XPGranted > 0 {
		s.notifier.XPGained(userID, progress.XPGranted, "quest:"+quest.ID, pet.XP())
	}

	if progress.Level > progress.PreviousLevel {
		s.notifier.LevelUp(userID, progress.Level)
	}
}

func (s *QuestService) actionCounts(
	ctx context.Context,
	userID uuid.UUID,
	now time.Time,
) (map[domain.Action]int, error) {
	from := domain.DayStart(now)
	aggregates, err := s.activity.XPByAction(ctx, userID, from, from.AddDate(0, 0, 1))
	if err != nil {
		return nil, err
	}

	counts := make(map[domain.Action]int, len(aggregates))
	for _, aggregate := range aggregates {
		counts[aggregate.Action] = aggregate.Count
	}

	return counts, nil
}

func questRewardAction(questID string) domain.Action {
	return domain.Action("quest_reward_" + questID)
}

func QuestSubjectID(userID uuid.UUID, questID string, now time.Time) uuid.UUID {
	sum := sha256.Sum256(
		[]byte(userID.String() + questID + domain.DayStart(now).Format(time.DateOnly)),
	)

	return uuid.NewSHA1(uuid.NameSpaceOID, sum[:])
}
