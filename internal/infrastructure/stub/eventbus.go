package stub

import (
	"context"

	"github.com/vladrzvk/weepost/internal/domain"
)

type NoOpEventBus struct{}

func NewNoOpEventBus() *NoOpEventBus { return &NoOpEventBus{} }

func (b *NoOpEventBus) Publish(_ context.Context, _ ...domain.DomainEvent) error {
	return nil
}

func (b *NoOpEventBus) PublishSystem(_ context.Context, _ ...domain.DomainEvent) error {
	return nil
}

func (b *NoOpEventBus) Subscribe(_ string, _ func(domain.DomainEvent)) error {
	return nil
}
