package events

import (
	"context"
	"sync"
)

type Outbox struct {
	mu      sync.Mutex
	pending []Event
}

func NewOutbox() *Outbox {
	return &Outbox{}
}

func (o *Outbox) Add(e Event) {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.pending = append(o.pending, e)
}

func (o *Outbox) Drain() []Event {
	o.mu.Lock()
	defer o.mu.Unlock()

	drained := o.pending
	o.pending = nil

	return drained
}

func (o *Outbox) Flush(ctx context.Context, p Publisher) {
	if p == nil {
		return
	}

	pending := o.Drain()
	for i := range pending {
		p.Publish(ctx, pending[i])
	}
}

func PublishAfterCommit(ctx context.Context, p Publisher, tx TxRunner, fn func(context.Context, *Outbox) error) error {
	outbox := NewOutbox()

	if err := tx.WithTx(ctx, func(ctx context.Context) error { return fn(ctx, outbox) }); err != nil {
		return err
	}

	outbox.Flush(ctx, p)

	return nil
}

type TxRunner interface {
	WithTx(ctx context.Context, fn func(context.Context) error) error
}
