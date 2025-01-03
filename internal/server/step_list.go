package server

import (
	"context"
	"fmt"
	"strings"

	"github.com/sergds/autovpn2/internal/rpc"
	"github.com/sergds/autovpn2/internal/server/executor"
)

// List our playbooks to the user
func (s *AutoVPNServer) StepList(updates chan *executor.ExecutorUpdate, ctx context.Context) context.Context {
	pbooks := GetAllPlaybooksFromDB(s.playbookDB)
	var pbnames []string = make([]string, 0)
	for pbname, _ := range pbooks {
		pbnames = append(pbnames, pbname)
	}
	updates <- &executor.ExecutorUpdate{CurrentStep: rpc.STEP_LIST, StepMessage: "Playbooks (" + fmt.Sprintf("%v", len(pbooks)) + "): " + strings.Join(pbnames, ", ")}
	return ctx
}
