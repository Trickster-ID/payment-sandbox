package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"payment-sandbox/app/modules/saga/models/entity"
)

type Orchestrator struct {
	DB *sql.DB
}

func NewOrchestrator(db *sql.DB) *Orchestrator { return &Orchestrator{DB: db} }

func (o *Orchestrator) Run(ctx context.Context, sagaType string, steps []entity.Step, payload map[string]any) error {
	id := uuid.New()
	body, _ := json.Marshal(payload)
	if _, err := o.DB.ExecContext(ctx, `
		INSERT INTO sagas (id, type, payload, status) VALUES ($1, $2, $3::jsonb, 'running')
	`, id, sagaType, string(body)); err != nil {
		return err
	}

	executed := []int{}
	for i, step := range steps {
		err := step.Execute(ctx, payload)
		o.logStep(ctx, id, i, step.Name(), "forward", err)
		if err != nil {
			o.update(ctx, id, "compensating", err.Error())
			for j := len(executed) - 1; j >= 0; j-- {
				idx := executed[j]
				cerr := steps[idx].Compensate(ctx, payload)
				o.logStep(ctx, id, idx, steps[idx].Name(), "compensate", cerr)
				if cerr != nil {
					o.update(ctx, id, "failed", fmt.Sprintf("compensation failed at step %d: %v", idx, cerr))
					return cerr
				}
			}
			o.update(ctx, id, "compensated", "")
			return err
		}
		executed = append(executed, i)
		_, _ = o.DB.ExecContext(ctx, `UPDATE sagas SET current_step=$1, updated_at=now() WHERE id=$2`, i+1, id)
	}
	o.update(ctx, id, "completed", "")
	return nil
}

func (o *Orchestrator) logStep(ctx context.Context, id uuid.UUID, idx int, name, direction string, err error) {
	status := "success"
	var errMsg *string
	if err != nil {
		status = "failed"
		s := err.Error()
		errMsg = &s
	}
	_, _ = o.DB.ExecContext(ctx, `
		INSERT INTO saga_step_log (saga_id, step_index, step_name, direction, status, error)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, id, idx, name, direction, status, errMsg)
}

func (o *Orchestrator) update(ctx context.Context, id uuid.UUID, status, lastErr string) {
	if lastErr == "" {
		_, _ = o.DB.ExecContext(ctx, `UPDATE sagas SET status=$1::saga_status, updated_at=now() WHERE id=$2`, status, id)
	} else {
		_, _ = o.DB.ExecContext(ctx, `UPDATE sagas SET status=$1::saga_status, last_error=$2, updated_at=now() WHERE id=$3`, status, lastErr, id)
	}
}
