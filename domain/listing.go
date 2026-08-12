package domain

const (
	ActionItemPublished Action = "item_published"
	ActionItemSold      Action = "item_sold"
	ActionItemUpdated   Action = "item_updated"
	ActionItemImproved  Action = "item_improved"
)

const (
	itemPublishedXP    = 50
	itemSoldXP         = 100
	itemUpdatedXP      = 5
	itemImprovedXP     = 15
	maxPublishesPerDay = 3
	publishSatiety     = 20
	soldMood           = 40
)

const (
	MaxUpdatesPerDay  = 5
	MaxImprovesPerDay = 3
	MaxSoldPerDay     = 5
)

func (p *Pet) RewardItemPublished(action LimitedAction) (Progress, error) {
	progress, err := p.rewardLimited(action, maxPublishesPerDay, itemPublishedXP)
	if err != nil {
		return Progress{}, err
	}

	p.Feed(publishSatiety, action.Now)

	return progress, nil
}

func (p *Pet) RewardItemSold(action LimitedAction) (Progress, error) {
	progress, err := p.rewardLimited(action, MaxSoldPerDay, itemSoldXP)
	if err != nil {
		return Progress{}, err
	}

	p.Cheer(soldMood, action.Now)

	return progress, nil
}

func (p *Pet) RewardItemUpdated(action LimitedAction) (Progress, error) {
	return p.rewardLimited(action, MaxUpdatesPerDay, itemUpdatedXP)
}

type ListingImprovement struct {
	PhotoAdded       bool
	DescriptionAdded bool
	PriceSet         bool
}

func (i ListingImprovement) Any() bool {
	return i.PhotoAdded || i.DescriptionAdded || i.PriceSet
}

func (p *Pet) RewardItemImproved(action LimitedAction, improvement ListingImprovement) (Progress, error) {
	if !improvement.Any() {
		return Progress{}, ErrConditionNotMet
	}

	return p.rewardLimited(action, MaxImprovesPerDay, itemImprovedXP)
}
