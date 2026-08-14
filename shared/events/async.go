package events

import (
	"context"
	"log/slog"
	"sync/atomic"
)

type AsyncTransport interface {
	Publish(ctx context.Context, e Event) error
}

type AsyncPublisher struct {
	transport AsyncTransport
	fallback  Publisher
	log       *slog.Logger
	degraded  atomic.Bool
}

func NewAsyncPublisher(transport AsyncTransport, fallback Publisher, log *slog.Logger) *AsyncPublisher {
	if log == nil {
		log = slog.Default()
	}

	return &AsyncPublisher{transport: transport, fallback: fallback, log: log}
}

func (p *AsyncPublisher) Publish(ctx context.Context, e Event) {
	if p.transport == nil {
		p.publishFallback(ctx, e)

		return
	}

	if err := p.transport.Publish(ctx, e); err != nil {
		if p.degraded.CompareAndSwap(false, true) {
			p.log.WarnContext(ctx, "async transport unavailable, falling back to in-process delivery",
				slog.String("event", string(e.Type)),
				slog.Any("error", err),
			)
		}
		p.publishFallback(ctx, e)

		return
	}

	if p.degraded.CompareAndSwap(true, false) {
		p.log.InfoContext(ctx, "async transport recovered")
	}
}

func (p *AsyncPublisher) Degraded() bool {
	return p.degraded.Load()
}

func (p *AsyncPublisher) publishFallback(ctx context.Context, e Event) {
	if p.fallback == nil {
		return
	}

	p.fallback.Publish(ctx, e)
}
