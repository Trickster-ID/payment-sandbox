package audit

import (
	"context"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
)

var redactedKeys = map[string]struct{}{
	"password": {}, "password_hash": {}, "cvv": {},
	"card_number": {}, "client_secret": {}, "authorization": {},
}

type noopLogger struct{}

func (n noopLogger) Log(_ context.Context, _ Event) error { return nil }

func NewNoopLogger() IAuditLogger { return noopLogger{} }

type Logger struct {
	coll *mongo.Collection
}

func NewLogger(db *mongo.Database) IAuditLogger {
	if db == nil {
		return noopLogger{}
	}
	return &Logger{coll: db.Collection("audit_events")}
}

func (l *Logger) Log(ctx context.Context, e Event) error {
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now().UTC()
	}
	redact(e.Before)
	redact(e.After)
	redact(e.Metadata)
	_, err := l.coll.InsertOne(ctx, e)
	return err
}

func redact(m map[string]any) {
	for k := range m {
		if _, ok := redactedKeys[strings.ToLower(k)]; ok {
			m[k] = "[REDACTED]"
		}
	}
}
