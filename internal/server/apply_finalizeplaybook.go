package server

import (
	"context"

	"github.com/sergds/autovpn2/internal/executor"
	"github.com/sergds/autovpn2/internal/playbook"
	"github.com/sergds/autovpn2/internal/rpc"
)

func (s *AutoVPNServer) StepFinalizePlaybook(updates chan *executor.ExecutorUpdate, ctx context.Context) context.Context {
	curpb := ctx.Value("playbook").(*playbook.Playbook)
	curpb.SetInstallState(true)
	curpb.Unlock()
	err := UpdatePlaybookDB(s.playbookDB, curpb)
	if err != nil {
		updates <- &executor.ExecutorUpdate{CurrentStep: rpc.STEP_ERROR, StepMessage: "Failed finalizing and updating playbook in db: " + err.Error()}
	}
	return ctx
}
