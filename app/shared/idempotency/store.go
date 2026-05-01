package idempotency

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

var ErrAlreadyExists = errors.New("idempotency key already claimed")

type Record struct {
	Key          string
	RequestHash  string
	Status       string
	ResponseCode int
	ResponseBody []byte
}

type Store struct {
	DB  *sql.DB
	TTL time.Duration
}

func (s *Store) Claim(ctx context.Context, key, userID, requestHash string) error {
	_, err := s.DB.ExecContext(ctx, `
		INSERT INTO idempotency_records (key, user_id, request_hash, status, expires_at)
		VALUES ($1, NULLIF($2,'')::uuid, $3, 'in_progress', now() + $4 * interval '1 second')
	`, key, userID, requestHash, int(s.TTL.Seconds()))
	if err != nil {
		return ErrAlreadyExists
	}
	return nil
}

func (s *Store) Fetch(ctx context.Context, key string) (*Record, error) {
	var r Record
	var bodyBytes []byte
	err := s.DB.QueryRowContext(ctx, `
		SELECT key, request_hash, status, COALESCE(response_code,0), COALESCE(response_body::text,'')
		FROM idempotency_records
		WHERE key = $1 AND expires_at > now()
	`, key).Scan(&r.Key, &r.RequestHash, &r.Status, &r.ResponseCode, &bodyBytes)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if len(bodyBytes) > 0 {
		var raw json.RawMessage
		_ = json.Unmarshal(bodyBytes, &raw)
		r.ResponseBody = raw
	}
	return &r, nil
}

func (s *Store) Complete(ctx context.Context, key string, code int, body []byte) error {
	_, err := s.DB.ExecContext(ctx, `
		UPDATE idempotency_records
		SET status='completed', response_code=$1, response_body=$2::jsonb, completed_at=now()
		WHERE key=$3
	`, code, string(body), key)
	return err
}
