package server

import (
	"context"
	"net"
	"slices"
	"strings"
	"time"

	"github.com/likexian/doh"
	"github.com/likexian/doh/dns"
	"github.com/sergds/autovpn2/internal/playbook"
	"github.com/sergds/autovpn2/internal/rpc"
	"github.com/sergds/autovpn2/internal/server/executor"
)

// Run DOH resolver to gather ips to route.
// Wants in context: "playbook"
func (s *AutoVPNServer) StepFetchIPs(updates chan *executor.ExecutorUpdate, ctx context.Context) context.Context {
	var dnsrecords map[string]string = make(map[string]string)
	curpb := ctx.Value("playbook").(*playbook.Playbook)
	for _, host := range curpb.Hosts {
		// Check if host is an internet address. Just store them as is and generate an arpa rdns domain.
		if net.ParseIP(host) != nil {
			octets := strings.Split(host, ".")
			slices.Reverse(octets)
			arpa := strings.Join(octets, ".") + ".in-addr.arpa"
			dnsrecords[arpa] = host
			updates <- &executor.ExecutorUpdate{CurrentStep: rpc.STEP_PUSH_SUMMARY, StepMessage: "Processed IP " + host + " -> " + arpa}

			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		c := doh.Use(doh.CloudflareProvider)
		resp, err := c.Query(ctx, dns.Domain(host), dns.TypeA)
		if err != nil {
			updates <- &executor.ExecutorUpdate{CurrentStep: rpc.STEP_ERROR, StepMessage: "Failed to resolve domain " + host + "! " + err.Error()}
			return ctx
		}
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
	curpb.PlaybookAddrs = dnsrecords
	ctx = context.WithValue(ctx, "playbook", curpb)
	ctx = context.WithValue(ctx, "dnsrecords", dnsrecords)
	return ctx
}
