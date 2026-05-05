package database

// Branch map for NewMongoDB (Section 3.1 of the plan):
//
// NewMongoDB(cfg):
// ├── cfg.MongoJourneyEnable is false       -> return nil, nil
// ├── mongo.Connect fails                   -> return nil, error (wrapped with "mongo connect:")
// ├── client.Ping fails                     -> return nil, error (wrapped with "mongo ping:"), client.Disconnect called
// └── all succeed                           -> return *mongo.Database, nil
//
// NOTE: Branches 2-4 require integration test with real MongoDB or complex mocking
// of concrete mongo.Connect function. Branch 1 is fully testable as a unit.

import (
	"testing"

	"payment-sandbox/app/config"

	"github.com/stretchr/testify/assert"
)

func TestNewMongoDB(t *testing.T) {
	type args struct {
		cfg config.Config
	}
	type wants struct {
		dbIsNil bool
		errMsg  string
	}

	tests := []struct {
		name  string
		args  args
		wants wants
	}{
		{
			name: "1. MongoJourneyEnable is false -> return nil, nil",
			args: args{cfg: config.Config{
				MongoJourneyEnable: false,
				MongoURI:           "mongodb://localhost:27017",
				MongoDBName:        "testdb",
			}},
			wants: wants{dbIsNil: true, errMsg: ""},
		},
		{
			name: "2. MongoJourneyEnable is true, invalid URI -> mongo connect error",
			args: args{cfg: config.Config{
				MongoJourneyEnable: true,
				MongoURI:           "invalid://uri",
				MongoDBName:        "testdb",
			}},
			wants: wants{dbIsNil: true, errMsg: "mongo connect:"},
		},
		{
			name: "3. MongoJourneyEnable is true, unreachable host -> mongo ping error",
			args: args{cfg: config.Config{
				MongoJourneyEnable: true,
				MongoURI:           "mongodb://invalid-host:27017",
				MongoDBName:        "testdb",
			}},
			wants: wants{dbIsNil: true, errMsg: "mongo ping:"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			db, err := NewMongoDB(tt.args.cfg)

			if tt.wants.dbIsNil {
				assert.Nil(t, db, "db should be nil")
				if tt.wants.errMsg != "" {
					assert.Error(t, err, "expected error")
					assert.Contains(t, err.Error(), tt.wants.errMsg, "error message contains expected text")
				}
			} else {
				assert.NoError(t, err, "unexpected error")
				assert.NotNil(t, db, "db should not be nil on success")
			}
		})
	}
}
