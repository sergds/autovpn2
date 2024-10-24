package server

import (
	"context"
	"strings"

	"github.com/sergds/autovpn2/internal/adapters/routes"
	"github.com/sergds/autovpn2/internal/executor"
	"github.com/sergds/autovpn2/internal/playbook"
	"github.com/sergds/autovpn2/internal/rpc"
)

// Put these routes on our router.
// Wants in context: playbook, dnsrecords
func (s *AutoVPNServer) StepApplyRoutes(updates chan *executor.ExecutorUpdate, ctx context.Context) context.Context {
	curpb := ctx.Value("playbook").(*playbook.Playbook)
	dnsrecords := ctx.Value("dnsrecords").(map[string]string)

	updates <- &executor.ExecutorUpdate{CurrentStep: rpc.STEP_PUSH_SUMMARY, StepMessage: "Routes Summary:"}
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
	cur_routes, err := routead.GetRoutes()
	if err != nil {
		updates <- &executor.ExecutorUpdate{CurrentStep: rpc.STEP_ERROR, StepMessage: "Failed to get routes from " + curpb.Adapters.Routes + ": " + err.Error()}
		return ctx
	}
	route_conflicts := make([]*routes.Route, 0)
	for _, r := range cur_routes {
		ip := strings.Split(r.Destination, "/")[0]
		for _, newip := range dnsrecords {
			if ip == newip && r.Interface == curpb.Interface {
				route_conflicts = append(route_conflicts, r)
			}
		}
	}
	if len(route_conflicts) != 0 {
		updates <- &executor.ExecutorUpdate{CurrentStep: rpc.STEP_PUSH_SUMMARY, StepMessage: "There are conflicts! The conflicting routes will be recreated!"}
		for _, r := range route_conflicts {
			err := routead.DelRoute(*r)
			if err != nil {
				updates <- &executor.ExecutorUpdate{CurrentStep: rpc.STEP_ERROR, StepMessage: "Failed to delete a route " + r.Destination + ": " + err.Error()}
				return ctx
			}
		}
	}
	for h, ip := range dnsrecords {
		err := routead.AddRoute(routes.Route{Destination: ip, Gateway: "0.0.0.0", Interface: curpb.Interface, Comment: "[AutoVPN2] Playbook: " + curpb.Name + " Host: " + h})
		if err != nil {
			updates <- &executor.ExecutorUpdate{CurrentStep: rpc.STEP_ERROR, StepMessage: "Failed to add a route " + ip + ": " + err.Error()}
			return ctx
		}
		updates <- &executor.ExecutorUpdate{CurrentStep: rpc.STEP_PUSH_SUMMARY, StepMessage: "Routed " + ip + "\t->\t" + curpb.Interface}
	}
	updates <- &executor.ExecutorUpdate{CurrentStep: rpc.STEP_ROUTES, StepMessage: "Saving changes"}
	routead.SaveConfig()
	return ctx
}
