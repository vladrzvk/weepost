package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// DomainEvent — in-process synchronous envelope (Phase 3 §1 — no broker V0)
// DA-3 Option A : WorkspaceID uuid.UUID champ obligatoire pour routage multi-tenant.
// Fields align to Phase 3 BaseEvent schema: event_id, event_type, workspace_id,
// aggregate_id, bounded_context, timestamp, data, correlation_id.
type DomainEvent struct {
	ID            string                 `json:"event_id"`
	EventType     string                 `json:"event_type"`
	WorkspaceID   uuid.UUID              `json:"workspace_id"`
	AggregateID   uuid.UUID              `json:"aggregate_id"`
	BoundedCtx    string                 `json:"bounded_context"`
	OccurredAt    time.Time              `json:"timestamp"`
	Version       int                    `json:"event_version"`
	Payload       map[string]interface{} `json:"data"`
	CorrelationID string                 `json:"correlation_id,omitempty"`
}

// IEventBus — in-process publish/subscribe; no message broker in V0
//
// SEC-01 asymmetry:
//   - Publish()       : business use cases — rejects events where WorkspaceID == uuid.Nil
//   - PublishSystem() : system/cross-tenant services only — accepts uuid.Nil WorkspaceID
//
// Only services explicitly typed as system-level (e.g. SecurityMonitoringService)
// may call PublishSystem(). Application use cases always use Publish().
type IEventBus interface {
	Publish(ctx context.Context, events ...DomainEvent) error
	PublishSystem(ctx context.Context, events ...DomainEvent) error
	// Subscribe registers a handler for the given event type.
	// In V0 in-process implementation, Subscribe never returns an error (handlers registered at startup).
	Subscribe(eventType string, handler func(DomainEvent)) error
}
