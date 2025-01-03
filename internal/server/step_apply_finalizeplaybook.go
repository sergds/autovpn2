package server

import (
	"context"
	"time"

	"github.com/sergds/autovpn2/internal/playbook"
	"github.com/sergds/autovpn2/internal/rpc"
	"github.com/sergds/autovpn2/internal/server/executor"
)

// Set out playbook as installed and unlock.
// Wants in context: "playbook"
func (s *AutoVPNServer) StepFinalizePlaybook(updates chan *executor.ExecutorUpdate, ctx context.Context) context.Context {
	curpb := ctx.Value("playbook").(*playbook.Playbook)
	curpb.SetInstallState(true)
	curpb.InstallTime = time.Now().Unix()
	curpb.Unlock()
	err := UpdatePlaybookDB(s.playbookDB, curpb)
	s.UpdateUpdaterTable()
	if err != nil {
		updates <- &executor.ExecutorUpdate{CurrentStep: rpc.STEP_ERROR, StepMessage: "Failed finalizing and updating playbook in db: " + err.Error()}
	}
	return ctx
}
