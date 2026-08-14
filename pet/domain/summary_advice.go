package domain

import "fmt"

var adviceTexts = map[ListingIssueKind]string{
	IssueNoPhoto:   `У объявления «%s» нет ни одной фотографии — с фото такие продаются заметно быстрее.`,
	IssueNoPrice:   `В объявлении «%s» не указана цена, покупатели такие обычно пролистывают.`,
	IssueShortText: `Описание у «%s» совсем короткое — пара строк про состояние и комплект сильно помогут.`,
	IssueStale:     `Объявление «%s» давно не обновлялось, подними его — покажется новым покупателям.`,
}

var adviceActions = map[ListingIssueKind]AdviceAction{
	IssueNoPhoto:   AdviceAddPhoto,
	IssueNoPrice:   AdviceSetPrice,
	IssueShortText: AdviceExpandText,
	IssueStale:     AdviceRefreshListing,
}

const noIssueAdviceText = "Объявления у тебя в порядке — заходи завтра, отметимся и продолжим серию."

func BuildAdvice(facts DayFacts) *Advice {
	issue, ok := facts.TopIssue()
	if !ok {
		return &Advice{Text: noIssueAdviceText, Action: AdviceCheckIn}
	}

	itemID := issue.ItemID

	return &Advice{
		Text:      fmt.Sprintf(adviceTexts[issue.Kind], issue.Title),
		ItemID:    &itemID,
		ItemTitle: issue.Title,
		Action:    adviceActions[issue.Kind],
	}
}
