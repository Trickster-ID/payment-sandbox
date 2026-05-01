package entity

import "context"

type Step interface {
	Name() string
	Execute(ctx context.Context, state map[string]any) error
	Compensate(ctx context.Context, state map[string]any) error
}
