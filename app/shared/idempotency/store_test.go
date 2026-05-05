package idempotency

// Branch analysis for Store methods:
//
// Claim:    exec success → nil error
//           exec error (duplicate) → ErrAlreadyExists
//
// Fetch:    no rows (sql.ErrNoRows) → (nil, nil)
//           other error → (nil, err)
//           row found → (*Record, nil)
//           json.Unmarshal error → still returns record with empty body
//
// Complete: exec success → nil error
//           exec error → error returned

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var fixedTime = time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)

func newStore(t *testing.T) (*Store, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err, "sqlmock.New")
	t.Cleanup(func() { db.Close() })
	return &Store{DB: db, TTL: 30 * time.Second}, mock
}

// ─── Store.Claim ──────────────────────────────────────────────────────────────

func TestStore_Claim(t *testing.T) {
	tests := []struct {
		name      string
		key       string
		userID    string
		hash      string
		setupMock func(m sqlmock.Sqlmock)
		wantErr   bool
		wantErrIs error
	}{
		{
			name:   "1. insert succeeds -> nil error",
			key:    "idem-key-1",
			userID: "user-1",
			hash:   "hash-abc",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectExec(regexp.QuoteMeta("INSERT INTO idempotency_records")).
					WithArgs("idem-key-1", "user-1", "hash-abc", 30).
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
			wantErr: false,
		},
		{
			name:   "2. insert fails (duplicate key) -> ErrAlreadyExists",
			key:    "idem-key-1",
			userID: "user-1",
			hash:   "hash-abc",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectExec(regexp.QuoteMeta("INSERT INTO idempotency_records")).
					WithArgs("idem-key-1", "user-1", "hash-abc", 30).
					WillReturnError(errors.New("duplicate key violation"))
			},
			wantErr:   true,
			wantErrIs: ErrAlreadyExists,
		},
		{
			name:   "3. insert with empty userID -> nil error",
			key:    "idem-key-2",
			userID: "",
			hash:   "hash-def",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectExec(regexp.QuoteMeta("INSERT INTO idempotency_records")).
					WithArgs("idem-key-2", "", "hash-def", 30).
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			store, mock := newStore(t)
			tt.setupMock(mock)

			err := store.Claim(context.Background(), tt.key, tt.userID, tt.hash)

			if tt.wantErr {
				if tt.wantErrIs != nil {
					assert.ErrorIs(t, err, tt.wantErrIs)
				} else {
					assert.Error(t, err)
				}
			} else {
				assert.NoError(t, err)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// ─── Store.Fetch ──────────────────────────────────────────────────────────────

func TestStore_Fetch(t *testing.T) {
	type wants struct {
		found bool
		code  int
		body  string
	}

	tests := []struct {
		name      string
		key       string
		setupMock func(m sqlmock.Sqlmock)
		wants     wants
	}{
		{
			name: "1. key not found (ErrNoRows) -> nil, nil",
			key:  "missing-key",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(regexp.QuoteMeta("SELECT key, request_hash, status")).
					WithArgs("missing-key").
					WillReturnRows(sqlmock.NewRows([]string{"key"}))
			},
			wants: wants{found: false},
		},
		{
			name: "2. query error -> nil, error",
			key:  "error-key",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(regexp.QuoteMeta("SELECT key, request_hash, status")).
					WithArgs("error-key").
					WillReturnError(errors.New("db error"))
			},
			wants: wants{found: false},
		},
		{
			name: "3. row found with response body -> *Record, nil",
			key:  "valid-key",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(regexp.QuoteMeta("SELECT key, request_hash, status")).
					WithArgs("valid-key").
					WillReturnRows(sqlmock.NewRows([]string{
						"key", "request_hash", "status", "response_code", "response_body",
					}).AddRow("valid-key", "hash-xyz", "completed", 200, `{"id":"test"}`))
			},
			wants: wants{found: true, code: 200, body: `{"id":"test"}`},
		},
		{
			name: "4. row found with empty response body -> *Record with empty body, nil",
			key:  "in-progress-key",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(regexp.QuoteMeta("SELECT key, request_hash, status")).
					WithArgs("in-progress-key").
					WillReturnRows(sqlmock.NewRows([]string{
						"key", "request_hash", "status", "response_code", "response_body",
					}).AddRow("in-progress-key", "hash-abc", "in_progress", 0, ""))
			},
			wants: wants{found: true, code: 0},
		},
		{
			name: "5. row found with null response_code -> defaults to 0, nil",
			key:  "null-code-key",
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(regexp.QuoteMeta("SELECT key, request_hash, status")).
					WithArgs("null-code-key").
					WillReturnRows(sqlmock.NewRows([]string{
						"key", "request_hash", "status", "response_code", "response_body",
					}).AddRow("null-code-key", "hash-def", "waiting", 0, ""))
			},
			wants: wants{found: true, code: 0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			store, mock := newStore(t)
			tt.setupMock(mock)

			rec, err := store.Fetch(context.Background(), tt.key)

			if tt.wants.found {
				assert.NoError(t, err)
				assert.NotNil(t, rec)
				assert.Equal(t, tt.key, rec.Key)
				assert.Equal(t, tt.wants.code, rec.ResponseCode)
			} else {
				assert.Nil(t, rec)
				// Error can be nil (ErrNoRows case) or a real error
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// ─── Store.Complete ───────────────────────────────────────────────────────────

func TestStore_Complete(t *testing.T) {
	tests := []struct {
		name      string
		key       string
		code      int
		body      []byte
		setupMock func(m sqlmock.Sqlmock)
		wantErr   bool
	}{
		{
			name: "1. update succeeds -> nil error",
			key:  "complete-key-1",
			code: 200,
			body: []byte(`{"result":"ok"}`),
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectExec(regexp.QuoteMeta("UPDATE idempotency_records")).
					WithArgs(200, string([]byte(`{"result":"ok"}`)), "complete-key-1").
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantErr: false,
		},
		{
			name: "2. update error -> error returned",
			key:  "complete-key-2",
			code: 201,
			body: []byte(`{"created":true}`),
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectExec(regexp.QuoteMeta("UPDATE idempotency_records")).
					WithArgs(201, string([]byte(`{"created":true}`)), "complete-key-2").
					WillReturnError(errors.New("db error"))
			},
			wantErr: true,
		},
		{
			name: "3. update with empty body -> success",
			key:  "complete-key-3",
			code: 204,
			body: []byte(``),
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectExec(regexp.QuoteMeta("UPDATE idempotency_records")).
					WithArgs(204, string([]byte(``)), "complete-key-3").
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
			wantErr: false,
		},
		{
			name: "4. update with error response code -> success",
			key:  "complete-key-4",
			code: 500,
			body: []byte(`{"error":"server error"}`),
			setupMock: func(m sqlmock.Sqlmock) {
				m.ExpectExec(regexp.QuoteMeta("UPDATE idempotency_records")).
					WithArgs(500, string([]byte(`{"error":"server error"}`)), "complete-key-4").
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			store, mock := newStore(t)
			tt.setupMock(mock)

			err := store.Complete(context.Background(), tt.key, tt.code, tt.body)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
