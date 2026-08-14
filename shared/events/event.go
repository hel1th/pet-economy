package events

import (
	"time"

	"github.com/google/uuid"
)

type Type string

const (
	TypeItemPublished  Type = "item.published"
	TypeItemSold       Type = "item.sold"
	TypeItemUpdated    Type = "item.updated"
	TypeFavoriteAdded  Type = "favorite.added"
	TypeItemViewed     Type = "item.viewed"
	TypeUserRegistered Type = "user.registered"
)

type Event struct {
	Type       Type
	UserID     uuid.UUID
	SubjectID  uuid.UUID
	OccurredAt time.Time
	Payload    Payload
}

const (
	AttrPhotoAdded       = "photo_added"
	AttrDescriptionAdded = "description_added"
	AttrPriceSet         = "price_set"
	AttrFlagTrue         = "true"
)

type Payload struct {
	Title       string
	Description string
	PriceKopeks int64
	PhotoCount  int
	HasVideo    bool
	Attributes  map[string]string
}

func (p Payload) Attribute(key string) (string, bool) {
	value, ok := p.Attributes[key]

	return value, ok
}

func New(t Type, userID, subjectID uuid.UUID, occurredAt time.Time) Event {
	return Event{Type: t, UserID: userID, SubjectID: subjectID, OccurredAt: occurredAt}
}

func (e Event) WithPayload(p Payload) Event {
	e.Payload = p

	return e
}
