package server

import (
	"context"
	"time"

	"github.com/likexian/doh"
	"github.com/likexian/doh/dns"
	"github.com/sergds/autovpn2/internal/executor"
	"github.com/sergds/autovpn2/internal/playbook"
	"github.com/sergds/autovpn2/internal/rpc"
)

func (s *AutoVPNServer) StepFetchIPs(updates chan *executor.ExecutorUpdate, ctx context.Context) context.Context {
	var dnsrecords map[string]string = make(map[string]string)
	curpb := ctx.Value("playbook").(*playbook.Playbook)
	for _, host := range curpb.Hosts {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		c := doh.Use(doh.CloudflareProvider)
		resp, err := c.Query(ctx, dns.Domain(host), dns.TypeA)
		answ := ""
		for _, a := range resp.Answer {
			if a.Type == 1 {
				answ = a.Data
			}
		}
		if err != nil {
			updates <- &executor.ExecutorUpdate{CurrentStep: rpc.STEP_PUSH_SUMMARY, StepMessage: "Failed to resolve domain " + host + "! " + err.Error()}
			continue
		}
		if answ != "" {
			dnsrecords[host] = answ
			updates <- &executor.ExecutorUpdate{CurrentStep: rpc.STEP_FETCHIP, StepMessage: "Resolved " + host + "\tIN\tA\t" + answ}
		} else {
			updates <- &executor.ExecutorUpdate{CurrentStep: rpc.STEP_PUSH_SUMMARY, StepMessage: "Failed getting INET Address of " + host + "!"}
			continue
		}
	}
	if curpb.Custom != nil {
		for h, ip := range curpb.Custom {
			dnsrecords[h] = ip
		}
	}
	ctx = context.WithValue(ctx, "dnsrecords", dnsrecords)
	return ctx
}
