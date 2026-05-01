package audit

import "time"

type Event struct {
	Timestamp  time.Time      `bson:"ts"`
	EventType  string         `bson:"event_type"`
	ActorID    string         `bson:"actor_id"`
	ActorType  string         `bson:"actor_type"`
	ResourceID string         `bson:"resource_id"`
	IPAddress  string         `bson:"ip,omitempty"`
	UserAgent  string         `bson:"ua,omitempty"`
	Before     map[string]any `bson:"before,omitempty"`
	After      map[string]any `bson:"after,omitempty"`
	Metadata   map[string]any `bson:"metadata,omitempty"`
	RequestID  string         `bson:"request_id"`
}
