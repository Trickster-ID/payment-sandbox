package services

import (
	"context"
	"log/slog"
	"time"
)

// StartRecoveryLoop runs every minute, scanning for sagas in running/compensating
// older than 5 minutes and logging them. Actual resume requires per-type step
// registry; extend per use case.
func (o *Orchestrator) StartRecoveryLoop(ctx context.Context) {
	go func() {
		t := time.NewTicker(time.Minute)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				rows, err := o.DB.QueryContext(ctx, `
					SELECT id, type, current_step
					FROM sagas
					WHERE status IN ('running','compensating')
					  AND updated_at < now() - interval '5 minutes'
				`)
				if err != nil {
					continue
				}
				for rows.Next() {
					var id, typ string
					var step int
					_ = rows.Scan(&id, &typ, &step)
					slog.Warn("stuck saga", "id", id, "type", typ, "current_step", step)
				}
				rows.Close()
			}
		}
	}()
}
