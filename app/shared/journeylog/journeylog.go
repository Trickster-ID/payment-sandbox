package journeylog

import (
	"context"
	"time"
)

type Event struct {
	EventID      string         `bson:"event_id"`
	OccurredAt   time.Time      `bson:"occurred_at"`
	RequestID    string         `bson:"request_id"`
	JourneyID    string         `bson:"journey_id"`
	ActorID      string         `bson:"actor_id"`
	ActorRole    string         `bson:"actor_role"`
	Module       string         `bson:"module"`
	EntityType   string         `bson:"entity_type"`
	EntityID     string         `bson:"entity_id"`
	Action       string         `bson:"action"`
	FromStatus   string         `bson:"from_status,omitempty"`
	ToStatus     string         `bson:"to_status,omitempty"`
	Result       string         `bson:"result"`
	ErrorCode    string         `bson:"error_code,omitempty"`
	ErrorMessage string         `bson:"error_message,omitempty"`
	Metadata     map[string]any `bson:"metadata,omitempty"`
}

type IJourneyLogger interface {
	Log(ctx context.Context, event Event) error
}
