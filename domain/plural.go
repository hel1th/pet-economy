package domain

import "fmt"

type pluralForms struct {
	one, few, many string
}

func plural(count int, forms pluralForms) string {
	value := count
	if value < 0 {
		value = -value
	}

	remainder100 := value % 100
	if remainder100 >= 11 && remainder100 <= 14 {
		return forms.many
	}

	switch value % 10 {
	case 1:
		return forms.one
	case 2, 3, 4:
		return forms.few
	default:
		return forms.many
	}
}

func pluralize(count int, forms pluralForms) string {
	return fmt.Sprintf("%d %s", count, plural(count, forms))
}

var (
	daysForms     = pluralForms{one: "день", few: "дня", many: "дней"}
	listingsForms = pluralForms{
		one:  "качественное объявление",
		few:  "качественных объявления",
		many: "качественных объявлений",
	}
	favoritesForms = pluralForms{one: "товар", few: "товара", many: "товаров"}
	itemsForms     = pluralForms{one: "объявление", few: "объявления", many: "объявлений"}
)
