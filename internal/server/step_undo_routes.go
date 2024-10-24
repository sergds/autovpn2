package server

import (
	"context"
	"strings"

	"github.com/sergds/autovpn2/internal/adapters/routes"
	"github.com/sergds/autovpn2/internal/executor"
	"github.com/sergds/autovpn2/internal/playbook"
	"github.com/sergds/autovpn2/internal/rpc"
)

// Remove these routes records from our router.
// Wants in context: "playbook"
func (*AutoVPNServer) StepUndoRoutes(updates chan *executor.ExecutorUpdate, ctx context.Context) context.Context {
	curpb := ctx.Value("playbook").(*playbook.Playbook)

	updates <- &executor.ExecutorUpdate{CurrentStep: rpc.UNDO_STEP_ROUTES, StepMessage: "Authenticating with " + curpb.Adapters.Routes + " route adapter..."}
	var routead routes.RouteAdapter = routes.NewRouteAdapter(curpb.Adapters.Routes)
	if routead == nil {
		updates <- &executor.ExecutorUpdate{CurrentStep: rpc.STEP_ERROR, StepMessage: "Failed to create route adapter " + curpb.Adapters.Routes}
		return ctx
	}
	err := routead.Authenticate(curpb.Adapterconfig.Routes)
	if err == nil {
		updates <- &executor.ExecutorUpdate{CurrentStep: rpc.STEP_PUSH_SUMMARY, StepMessage: "Authenticated!"}
	} else {
		updates <- &executor.ExecutorUpdate{CurrentStep: rpc.STEP_ERROR, StepMessage: "Failed to authenticate on " + curpb.Adapters.Routes + ": " + err.Error()}
		return ctx
	}

	updates <- &executor.ExecutorUpdate{CurrentStep: rpc.UNDO_STEP_ROUTES, StepMessage: "Trying to get addresses from route addresses"}
	var addrs []string = make([]string, 0)
	cur_routes, err := routead.GetRoutes()
	if err != nil || len(cur_routes) == 0 {
		for _, ip := range curpb.PlaybookAddrs {
			addrs = append(addrs, ip)
		}
		updates <- &executor.ExecutorUpdate{CurrentStep: rpc.STEP_PUSH_SUMMARY, StepMessage: "Falling back to address cold storage!"}
	} else {
		updates <- &executor.ExecutorUpdate{CurrentStep: rpc.STEP_PUSH_SUMMARY, StepMessage: "Retrieved needed addresses from router adapter!"}
		for _, r := range cur_routes {
			if strings.Contains(r.Comment, "AutoVPN2") {
				if strings.Contains(r.Comment, curpb.Name) {
					addrs = append(addrs, r.Destination)
				}
			}
		}
	}
	for _, ip := range addrs {
		err := routead.DelRoute(routes.Route{Destination: ip, Gateway: "0.0.0.0", Interface: curpb.Interface})
		if err != nil {
			updates <- &executor.ExecutorUpdate{CurrentStep: rpc.STEP_PUSH_SUMMARY, StepMessage: "Failed to unroute: " + ip}
		}
		updates <- &executor.ExecutorUpdate{CurrentStep: rpc.STEP_PUSH_SUMMARY, StepMessage: "Unrouted " + ip}
	}
	routead.SaveConfig()
	return ctx
}
