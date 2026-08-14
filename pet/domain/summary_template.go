package domain

import (
	"fmt"
	"hash/fnv"
	"strings"
	"unicode/utf8"
)

var emptyDayOpenings = []string{
	"Скучал по тебе весь день.",
	"Сегодня было тихо — я ждал тебя у окна.",
	"Ты не заходил, и день получился совсем пустой.",
}

var emptyDayClosings = []string{
	"Загляни завтра — вместе наверстаем.",
	"Приходи завтра, у меня накопилось много идей.",
	"Жду тебя завтра, обещаю не грустить.",
}

var activeDayOpenings = []string{
	"Хороший вышел день!",
	"Ну и денёк у нас с тобой был!",
	"День удался, я доволен.",
}

var levelUpPhrases = []string{
	"я подрос до %d уровня",
	"мы дотянули до %d уровня",
	"я вырос — теперь %d уровень",
}

var xpPhrases = []string{
	"набрал %d XP",
	"заработал %d XP",
	"накопил %d XP",
}

var streakPhrases = []string{
	"серия держится уже %s подряд",
	"мы вместе %s без пропусков",
	"%s серии — это сила",
}

var streakBrokenPhrases = []string{
	"серия оборвалась, но это не страшно — начнём заново",
	"серия сгорела, зато начинаем с чистого листа",
	"серия обнулилась, ничего, соберём новую",
}

var hungryPhrases = []string{
	"Я немного проголодался",
	"В животе урчит",
	"Сытость просела",
}

func BuildSummaryMessage(facts DayFacts) string {
	seed := templateSeed(facts)

	if facts.IsEmpty() {
		return joinSentences(
			pick(emptyDayOpenings, seed),
			petStateSentence(facts, seed),
			pick(emptyDayClosings, seed),
		)
	}

	return joinSentences(
		pick(activeDayOpenings, seed),
		achievementSentence(facts, seed),
		streakSentence(facts, seed),
		leaderboardSentence(facts),
	)
}

func achievementSentence(facts DayFacts, seed uint32) string {
	parts := make([]string, 0, 3)

	if activity := activitySentence(facts); activity != "" {
		parts = append(parts, activity)
	}
	if facts.TotalXP > 0 {
		parts = append(parts, fmt.Sprintf(pick(xpPhrases, seed), facts.TotalXP))
	}
	if facts.LeveledUp {
		parts = append(parts, fmt.Sprintf(pick(levelUpPhrases, seed), facts.Level.Current))
	}
	if len(parts) == 0 {
		return "Ты заглянул ко мне, и этого уже достаточно"
	}

	return "Ты " + strings.Join(parts, ", ")
}

var actionLabels = map[Action]struct {
	template string
	forms    pluralForms
}{
	ActionQualityListing: {"оформил %s", listingsForms},
	ActionFavorite:       {"добавил %s в избранное", favoritesForms},
	ActionItemViewed:     {"посмотрел %s", itemsForms},
	ActionItemPublished:  {"опубликовал %s", itemsForms},
	ActionItemUpdated:    {"обновил %s", itemsForms},
	ActionItemImproved:   {"улучшил %s", itemsForms},
	ActionItemSold:       {"продал %s", itemsForms},
}

func activitySentence(facts DayFacts) string {
	best := XPByAction{}
	for _, entry := range facts.Actions {
		if _, ok := actionLabels[entry.Action]; !ok {
			continue
		}
		if entry.Amount > best.Amount || (entry.Amount == best.Amount && entry.Count > best.Count) {
			best = entry
		}
	}
	if best.Count == 0 {
		return ""
	}

	label := actionLabels[best.Action]

	return fmt.Sprintf(label.template, pluralize(best.Count, label.forms))
}

func streakSentence(facts DayFacts, seed uint32) string {
	switch {
	case facts.Streak.Broken:
		return capitalize(pick(streakBrokenPhrases, seed))
	case facts.Streak.Days > 1:
		return capitalize(fmt.Sprintf(pick(streakPhrases, seed), pluralize(facts.Streak.Days, daysForms)))
	default:
		return ""
	}
}

func leaderboardSentence(facts DayFacts) string {
	switch {
	case facts.RankImproved():
		return fmt.Sprintf("В лидерборде поднялись с %d на %d место",
			facts.Leaderboard.Previous, facts.Leaderboard.Rank)
	case facts.RankDropped():
		return fmt.Sprintf("В лидерборде мы теперь %d — нас догоняют",
			facts.Leaderboard.Rank)
	case facts.Leaderboard.Known:
		return fmt.Sprintf("Держим %d место в лидерборде", facts.Leaderboard.Rank)
	default:
		return ""
	}
}

func petStateSentence(facts DayFacts, seed uint32) string {
	if facts.Pet.Satiety < sadSatietyThreshold {
		return pick(hungryPhrases, seed) + ", но я справляюсь"
	}
	if facts.Streak.Days > 1 {
		return fmt.Sprintf("Серия пока держится — %s", pluralize(facts.Streak.Days, daysForms))
	}

	return ""
}

func joinSentences(parts ...string) string {
	kept := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		kept = append(kept, terminate(trimmed))
	}

	return strings.Join(kept, " ")
}

func terminate(sentence string) string {
	if strings.HasSuffix(sentence, "!") || strings.HasSuffix(sentence, "?") {
		return sentence
	}

	return strings.TrimSuffix(sentence, ".") + "."
}

func pick(options []string, seed uint32) string {
	return options[int(seed)%len(options)]
}

func templateSeed(facts DayFacts) uint32 {
	hasher := fnv.New32a()
	_, _ = hasher.Write([]byte(facts.Date.Format("2006-01-02") + facts.PetName))

	return hasher.Sum32()
}

func capitalize(value string) string {
	runes := []rune(value)
	if len(runes) == 0 {
		return value
	}
	upper, _ := utf8.DecodeRuneInString(strings.ToUpper(string(runes[0])))
	runes[0] = upper

	return string(runes)
}
