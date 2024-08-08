package server

import (
	"context"
	"net"
	"time"

	dnsadapters "github.com/sergds/autovpn2/internal/adapters/dns"
	"github.com/sergds/autovpn2/internal/executor"
	"github.com/sergds/autovpn2/internal/playbook"
	"github.com/sergds/autovpn2/internal/rpc"
)

func (s *AutoVPNServer) StepApplyDNS(updates chan *executor.ExecutorUpdate, ctx context.Context) context.Context {
	curpb := ctx.Value("playbook").(*playbook.Playbook)
	dnsrecords := ctx.Value("dnsrecords").(map[string]string)

	updates <- &executor.ExecutorUpdate{CurrentStep: rpc.STEP_PUSH_SUMMARY, StepMessage: "DNS Summary:"}
	var dnsad dnsadapters.DNSAdapter = dnsadapters.NewDNSAdapter(curpb.Adapters.Dns)
	if err := dnsad.Authenticate(curpb.Adapterconfig.Dns); err == nil {
		updates <- &executor.ExecutorUpdate{CurrentStep: rpc.STEP_PUSH_SUMMARY, StepMessage: "Authenticated!"}
	} else {
		updates <- &executor.ExecutorUpdate{CurrentStep: rpc.STEP_ERROR, StepMessage: "Unauthorized!"}
		time.Sleep(1 * time.Second)
		return ctx
	}
	for host, ip := range dnsrecords {
		ipaddr := net.ParseIP(ip)
		err := dnsad.AddRecord(dnsadapters.DNSRecord{Domain: host, Addr: ipaddr, Type: "A"})
		if err != nil {
			updates <- &executor.ExecutorUpdate{CurrentStep: rpc.STEP_PUSH_SUMMARY, StepMessage: "Failed to add " + host + "\tIN\tA\t" + ip + ": " + err.Error()}
			return ctx
		}
		updates <- &executor.ExecutorUpdate{CurrentStep: rpc.STEP_PUSH_SUMMARY, StepMessage: "Added " + host + "\tIN\tA\t" + ip}
	}
	err := UpdatePlaybookDB(s.playbookDB, curpb)
	if err != nil {
		updates <- &executor.ExecutorUpdate{CurrentStep: rpc.STEP_ERROR, StepMessage: "Failed updating playbook in db: " + err.Error()}
	}
	dnsad.CommitRecords()
	return ctx

}
