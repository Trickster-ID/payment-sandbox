package audit

import "context"

//go:generate mockery --name IAuditLogger --output mocks --outpkg mocks --case underscore

type IAuditLogger interface {
	Log(ctx context.Context, event Event) error
}
