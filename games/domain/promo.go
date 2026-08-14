package domain

import (
	"strings"

	"github.com/hel1th/pet-economy/shared/publicid"
)

const (
	promoPrefix = "PROMO-"
	promoLength = 8
)

func NewPromoCode() string {
	return promoPrefix + strings.ToUpper(publicid.New()[:promoLength])
}
