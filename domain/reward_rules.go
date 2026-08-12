package domain

type RewardProgress struct {
	Reward   Reward
	Unlocked bool
	Current  int
	Target   int
}

func RewardConditionMet(reward Reward, pet *Pet) bool {
	if pet == nil {
		return false
	}

	current, ok := conditionCurrent(reward.ConditionType(), pet)
	if !ok {
		return false
	}

	return current >= reward.ConditionValue()
}

func EvaluateRewardProgress(reward Reward, pet *Pet) RewardProgress {
	current, _ := conditionCurrent(reward.ConditionType(), pet)

	return RewardProgress{
		Reward:   reward,
		Unlocked: RewardConditionMet(reward, pet),
		Current:  current,
		Target:   reward.ConditionValue(),
	}
}

func EligibleRewards(rewards []Reward, pet *Pet) []Reward {
	eligible := make([]Reward, 0, len(rewards))
	for _, reward := range rewards {
		if RewardConditionMet(reward, pet) {
			eligible = append(eligible, reward)
		}
	}

	return eligible
}

func RewardIDForLevel(level int) string {
	if level < 1 || level > MaxLevel {
		return ""
	}

	return levelRewards[level]
}

func conditionCurrent(condition ConditionType, pet *Pet) (int, bool) {
	if pet == nil {
		return 0, false
	}

	switch condition {
	case ConditionLevel:
		return pet.Level(), true
	case ConditionStreak:
		return pet.StreakDays(), true
	case ConditionAchievement:
		return 0, false
	default:
		return 0, false
	}
}
