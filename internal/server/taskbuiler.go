package server

import (
	"context"

	"github.com/sergds/autovpn2/internal/playbook"
	"github.com/sergds/autovpn2/internal/rpc"
	"github.com/sergds/autovpn2/internal/server/executor"
)

// Builds a task, by creating an executor with a specific set of steps and a prepared context.
type TaskBuilder struct {
	serv *AutoVPNServer
	exec *executor.Executor
}

func NewTaskBuilder(srv *AutoVPNServer) *TaskBuilder {
	return &TaskBuilder{exec: executor.NewExecutor(), serv: srv}
}

func (tb *TaskBuilder) List() error {
	tb.exec.AddStep(executor.NewStep(rpc.STEP_LIST, tb.serv.StepList))
	return nil
}

func (tb *TaskBuilder) Apply(playbk_yaml string) error {
	// Build context for executor
	var is_updated = false
	currpc, err := playbook.Parse(playbk_yaml)
	if err != nil {
		return err
	}
	ctx := context.WithValue(context.Background(), "playbook", currpc)
	rpcooks := GetAllPlaybooksFromDB(tb.serv.playbookDB)
	for pname, rpcook := range rpcooks {
		if currpc.Name == pname && rpcook.GetInstallState() {
			is_updated = true
			ctx = context.WithValue(ctx, "old_playbook", rpcook)
		}
	}
	tb.exec.SetContext(ctx)
	tb.exec.AddStep(executor.NewStep(rpc.STEP_LOCK_ADD, tb.serv.StepApplyLockAdd))
	// Just to be sure. Apply steps ~should~ handle old addrs, but potentially can lead to stray routes or dns records in the long run when undoing time cometb.serv.
	if is_updated {
		tb.exec.AddStep(executor.NewStep("swap", tb.serv.StepSwapPlaybooks))
		tb.exec.AddStep(executor.NewStep(rpc.UNDO_STEP_DNS, tb.serv.StepUndoDNS))
		tb.exec.AddStep(executor.NewStep(rpc.UNDO_STEP_ROUTES, tb.serv.StepUndoRoutes))
		tb.exec.AddStep(executor.NewStep("swap", tb.serv.StepSwapPlaybooks)) // At this point old_playbook is no longer needed and was overwritten, so no need to unlock and stuff.
		// Continue with new one as usual.
	}
	tb.exec.AddStep(executor.NewStep(rpc.STEP_FETCHIP, tb.serv.StepFetchIPs))
	tb.exec.AddStep(executor.NewStep(rpc.STEP_DNS, tb.serv.StepApplyDNS))
	tb.exec.AddStep(executor.NewStep(rpc.STEP_DNS, tb.serv.StepUpdatePlaybook))
	tb.exec.AddStep(executor.NewStep(rpc.STEP_ROUTES, tb.serv.StepApplyRoutes))
	tb.exec.AddStep(executor.NewStep(rpc.STEP_ROUTES, tb.serv.StepFinalizePlaybook)) // "finalize" here - set status as installed and unlock
	return nil
}

func (tb *TaskBuilder) Undo(pbook_name string) error {
	tb.exec.AddStep(executor.NewStep("prep_ctx", func(updates chan *executor.ExecutorUpdate, ctx context.Context) context.Context { // TODO: Should I introduce new step const for these?
		var ok bool = false
		var wasinstalled bool = false
		var curpb *playbook.Playbook = nil
		pbooks := GetAllPlaybooksFromDB(tb.serv.playbookDB)

		for _, pbook := range pbooks {
			if pbook.Name == pbook_name {
				ok = true
				curpb = pbook
				if pbook.GetInstallState() {
					wasinstalled = true
				}
			}
		}
		if !ok {
			updates <- &executor.ExecutorUpdate{CurrentStep: rpc.STEP_ERROR, StepMessage: "No such playbook " + pbook_name + " installed!"}
			return ctx
		}
		if !wasinstalled {
			updates <- &executor.ExecutorUpdate{CurrentStep: rpc.STEP_ERROR, StepMessage: "Such playbook exists, but didn't finish installing! Removing!"}
			DeletePlaybookDB(tb.serv.playbookDB, curpb)
			return ctx
		}
		if !curpb.Lock("Undo") {
			updates <- &executor.ExecutorUpdate{CurrentStep: rpc.STEP_ERROR, StepMessage: "Playbook is being processed at the moment (reason: " + curpb.GetLockReason() + ")!"}
			return ctx
		}
		err := UpdatePlaybookDB(tb.serv.playbookDB, curpb)
		if err != nil {
			updates <- &executor.ExecutorUpdate{CurrentStep: rpc.STEP_ERROR, StepMessage: "Failed updating playbook in db: " + err.Error()}
			return ctx
		}
		tb.serv.UpdateUpdaterTable()
		ctx = context.WithValue(ctx, "playbook", curpb)
		return ctx
	}))
	tb.exec.AddStep(executor.NewStep(rpc.UNDO_STEP_DNS, tb.serv.StepUpdatePlaybook))
	tb.exec.AddStep(executor.NewStep(rpc.UNDO_STEP_DNS, tb.serv.StepUndoDNS))
	tb.exec.AddStep(executor.NewStep(rpc.UNDO_STEP_ROUTES, tb.serv.StepUndoRoutes))
	tb.exec.AddStep(executor.NewStep("finalize", func(updates chan *executor.ExecutorUpdate, ctx context.Context) context.Context {
		curpb := ctx.Value("playbook").(*playbook.Playbook)
		err := DeletePlaybookDB(tb.serv.playbookDB, curpb)
		if err != nil {
			updates <- &executor.ExecutorUpdate{CurrentStep: rpc.STEP_ERROR, StepMessage: "Failed removing playbook from db: " + err.Error()}
		}
		return ctx
	}))
	return nil
}

func (tb *TaskBuilder) Build() *executor.Executor {
	return tb.exec
}
