package database

// Branch map for New (Section 3.1 of the plan):
//
// New(cfg):
// ├── sql.Open fails (invalid DSN format)        -> return nil, error
// ├── db.Ping fails (host unreachable)           -> return nil, error
// └── all succeed                                -> return *sql.DB, nil
//
// NOTE: sql.Open error and success branches require integration test with real PostgreSQL.
// The Ping error branch is tested below with an unreachable host.

import (
	"testing"

	"payment-sandbox/app/config"

	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	type args struct {
		cfg config.Config
	}
	type wants struct {
		dbIsNil bool
		hasErr  bool
	}

	tests := []struct {
		name  string
		args  args
		wants wants
	}{
		{
			name: "1. invalid host -> Ping error",
			args: args{cfg: config.Config{
				DBHost:     "invalid-host-that-does-not-exist",
				DBPort:     "5432",
				DBUser:     "user",
				DBPassword: "password",
				DBName:     "testdb",
				DBSSLMode:  "disable",
			}},
			wants: wants{dbIsNil: true, hasErr: true},
		},
		{
			name: "2. invalid port format -> Open error",
			args: args{cfg: config.Config{
				DBHost:     "localhost",
				DBPort:     "invalid-port",
				DBUser:     "user",
				DBPassword: "password",
				DBName:     "testdb",
				DBSSLMode:  "disable",
			}},
			wants: wants{dbIsNil: true, hasErr: true},
		},
		{
			name: "3. empty host -> Ping error",
			args: args{cfg: config.Config{
				DBHost:     "",
				DBPort:     "5432",
				DBUser:     "user",
				DBPassword: "password",
				DBName:     "testdb",
				DBSSLMode:  "disable",
			}},
			wants: wants{dbIsNil: true, hasErr: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			db, err := New(tt.args.cfg)

			if tt.wants.hasErr {
				assert.Error(t, err, "expected error")
				assert.Nil(t, db, "db should be nil on error")
			} else {
				assert.NoError(t, err, "unexpected error")
				assert.NotNil(t, db, "db should not be nil on success")
				if db != nil {
					err := db.Close()
					assert.NoError(t, err, "close db connection")
				}
			}
		})
	}
}
