package executor

import "context"

type Step struct {
	Id string
	F  func(updates chan string, ctx context.Context) context.Context
}

func NewStep(id string, f func(updates chan string, ctx context.Context) context.Context) *Step {
	return &Step{Id: id, F: f}
}

func (s *Step) Exec(ctx context.Context, updates chan string) context.Context {
	return s.F(updates, ctx)
}
