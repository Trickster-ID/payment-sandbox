package journeylog

import (
	"context"
	"log"
	"time"

	"payment-sandbox/app/config"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

type noopLogger struct{}

func (n noopLogger) Log(_ context.Context, _ Event) error {
	return nil
}

func NewNoopJourneyLogger() IJourneyLogger {
	return noopLogger{}
}

type mongoLogger struct {
	collection *mongo.Collection
}

func NewMongoJourneyLogger(cfg config.Config) IJourneyLogger {
	if !cfg.MongoJourneyEnable {
		return noopLogger{}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(cfg.MongoURI))
	if err != nil {
		log.Printf("journey logger disabled: mongodb connect failed: %v", err)
		return noopLogger{}
	}
	if err := client.Ping(ctx, readpref.Primary()); err != nil {
		log.Printf("journey logger disabled: mongodb ping failed: %v", err)
		_ = client.Disconnect(context.Background())
		return noopLogger{}
	}

	return &mongoLogger{
		collection: client.Database(cfg.MongoDBName).Collection(cfg.MongoCollection),
	}
}

func (m *mongoLogger) Log(ctx context.Context, event Event) error {
	if event.EventID == "" {
		event.EventID = uuid.NewString()
	}
	if event.OccurredAt.IsZero() {
		event.OccurredAt = time.Now().UTC()
	}

	writeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	_, err := m.collection.InsertOne(writeCtx, event)
	return err
}
