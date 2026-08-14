package events

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime/debug"
	"sync"
)

var ErrHandlerPanic = errors.New("event handler panicked")

type Handler func(ctx context.Context, e Event) error

type Publisher interface {
	Publish(ctx context.Context, e Event)
}

type Subscriber interface {
	Subscribe(t Type, h Handler)
}

type Bus struct {
	mu       sync.RWMutex
	handlers map[Type][]Handler
	log      *slog.Logger
}

func NewBus(log *slog.Logger) *Bus {
	if log == nil {
		log = slog.Default()
	}

	return &Bus{handlers: make(map[Type][]Handler), log: log}
}

func (b *Bus) Subscribe(t Type, h Handler) {
	if h == nil {
		return
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	b.handlers[t] = append(b.handlers[t], h)
}

func (b *Bus) Publish(ctx context.Context, e Event) {
	for _, h := range b.snapshot(e.Type) {
		if err := b.deliver(ctx, h, e); err != nil {
			b.log.ErrorContext(ctx, "event handler failed",
				slog.String("event", string(e.Type)),
				slog.String("user_id", e.UserID.String()),
				slog.Any("error", err),
			)
		}
	}
}

func (b *Bus) deliver(ctx context.Context, h Handler, e Event) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("%w: %v", ErrHandlerPanic, recovered)
			b.log.ErrorContext(ctx, "event handler panicked",
				slog.String("event", string(e.Type)),
				slog.String("stack", string(debug.Stack())),
			)
		}
	}()

	return h(ctx, e)
}

func (b *Bus) snapshot(t Type) []Handler {
	b.mu.RLock()
	defer b.mu.RUnlock()

	handlers := make([]Handler, len(b.handlers[t]))
	copy(handlers, b.handlers[t])

	return handlers
}

type NopPublisher struct{}

func (NopPublisher) Publish(context.Context, Event) {}
