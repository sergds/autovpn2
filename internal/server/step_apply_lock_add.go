package server

import (
	"context"

	"github.com/sergds/autovpn2/internal/playbook"
	pb "github.com/sergds/autovpn2/internal/rpc"
	"github.com/sergds/autovpn2/internal/server/executor"
)

// Lock newly parsed playbook and add to db.
// Wants in context: playbook
func (s *AutoVPNServer) StepApplyLockAdd(updates chan *executor.ExecutorUpdate, ctx context.Context) context.Context {
	curpb := ctx.Value("playbook").(*playbook.Playbook)
	if !curpb.Lock("Apply") {
		updates <- &executor.ExecutorUpdate{CurrentStep: pb.STEP_ERROR, StepMessage: "Unexpected lock on fresh playbook! (reason: " + curpb.GetLockReason() + ")"}
		return ctx
	}
	err := UpdatePlaybookDB(s.playbookDB, curpb)
	if err != nil {
		updates <- &executor.ExecutorUpdate{CurrentStep: pb.STEP_ERROR, StepMessage: "Failed adding playbook to db: " + err.Error()}
		return ctx
	}
	ctx = context.WithValue(ctx, "playbook", curpb)
	return ctx
}
