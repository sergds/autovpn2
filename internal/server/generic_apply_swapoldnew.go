package server

import (
	"context"

	"github.com/sergds/autovpn2/internal/executor"
	"github.com/sergds/autovpn2/internal/playbook"
	pb "github.com/sergds/autovpn2/internal/rpc"
)

// Swap new playbook with retrieved old. For updating existing playbooks with new revisions.
func (s *AutoVPNServer) StepSwapPlaybooks(updates chan *executor.ExecutorUpdate, ctx context.Context) context.Context {
	old_pbook := ctx.Value("old_playbook").(*playbook.Playbook)
	pbook := ctx.Value("playbook").(*playbook.Playbook)

	// Pre swap run -- lock old playbook, so that auto-update doesn't get in the way.
	if old_pbook.GetLockReason() != "Reapply" && old_pbook.GetLockReason() != "Apply" { // Make sure that it isn't locked already or actually is the new one post-swap.
		if old_pbook.Lock("Reapply") {
			err := UpdatePlaybookDB(s.playbookDB, old_pbook)
			if err != nil {
				updates <- &executor.ExecutorUpdate{CurrentStep: pb.STEP_ERROR, StepMessage: "Failed locking old playbook in db: " + err.Error()}
				return ctx
			}
		}
	}

	ctx = context.WithValue(ctx, "old_playbook", pbook)
	ctx = context.WithValue(ctx, "playbook", old_pbook)
	return ctx
}
