package executor

import "context"

// Like they say: if you break big tasks into smaller ones, then everything's achievable. So step is that one discrete part of a bigger task.
// You make one with NewStep and feed it to executor. That's it. It's just a freaking fancy wrapper for a function, idk what to document there...
type Step struct {
	Id string
	F  func(updates chan *ExecutorUpdate, ctx context.Context) context.Context
}

func NewStep(id string, f func(updates chan *ExecutorUpdate, ctx context.Context) context.Context) *Step {
	return &Step{Id: id, F: f}
}

func (s *Step) Exec(ctx context.Context, updates chan *ExecutorUpdate) context.Context {
	return s.F(updates, ctx)
}
