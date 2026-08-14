package domain

const MaxLevel = 15

var levelThresholds = [...]int{
	0, 0, 5, 12, 22, 35, 52, 72, 95, 122, 155, 195, 240, 290, 350, 420,
}

var levelRewards = [...]string{
	0:  "",
	1:  "starter_status",
	2:  "raccoon_accessory",
	3:  "services_discount_10",
	4:  "attentive_badge",
	5:  "listing_badge_or_delivery_discount",
	6:  "raccoon_avatar_frame",
	7:  "favorites_personal_offer",
	8:  "dialogue_master_badge",
	9:  "autoteka_discount_30",
	10: "free_delivery_500",
	11: "xl_listing_discount_50",
	12: "treasure_hunter_badge",
	13: "views_boost_24h",
	14: "raccoon_sticker_pack",
	15: "free_delivery_and_avito_guru_badge",
}

type Progress struct {
	XPGranted       int
	PreviousLevel   int
	Level           int
	NextLevelXP     int
	IsMaxLevel      bool
	UnlockedRewards []string
}

func (p *Pet) awardXP(amount int) Progress {
	previousLevel := p.level
	p.xp += amount

	for p.level < MaxLevel && p.xp >= levelThresholds[p.level+1] {
		p.level++
	}
	p.nextLevelXP = nextThreshold(p.level)
	if p.hatchedAt != nil {
		p.stage = stageForLevel(p.level)
	}

	rewards := make([]string, 0, p.level-previousLevel)
	for level := previousLevel + 1; level <= p.level; level++ {
		rewards = append(rewards, levelRewards[level])
	}

	return Progress{
		XPGranted: amount, PreviousLevel: previousLevel, Level: p.level,
		NextLevelXP: p.nextLevelXP, IsMaxLevel: p.IsMaxLevel(), UnlockedRewards: rewards,
	}
}

func nextThreshold(level int) int {
	if level >= MaxLevel {
		return 0
	}

	return levelThresholds[level+1]
}

func stageForLevel(level int) Stage {
	switch {
	case level >= MaxLevel:
		return StageLegend
	case level >= 10:
		return StageAdult
	case level >= 6:
		return StageTeen
	default:
		return StageBaby
	}
}
