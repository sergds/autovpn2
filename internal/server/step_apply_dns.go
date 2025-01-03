package server

import (
	"context"
	"net"
	"strings"
	"time"

	dnsadapters "github.com/sergds/autovpn2/internal/adapters/dns"
	"github.com/sergds/autovpn2/internal/playbook"
	"github.com/sergds/autovpn2/internal/rpc"
	"github.com/sergds/autovpn2/internal/server/executor"
)

// Put these DNS records onto our dns cache server or whatever.
// Wants in context: "playbook", "dnsrecords"
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
	conflicts := make([]dnsadapters.DNSRecord, 0)
	recs, err := dnsad.GetRecords("A")
	if err != nil {
		updates <- &executor.ExecutorUpdate{CurrentStep: rpc.STEP_PUSH_SUMMARY, StepMessage: "Failed getting conflicts! Applying blindly"}
	}
	for _, rec := range recs {
		for _, domain := range curpb.Hosts {
			if rec.Domain == domain {
				updates <- &executor.ExecutorUpdate{CurrentStep: rpc.STEP_PUSH_SUMMARY, StepMessage: "Found conflicting record: " + rec.Domain}
				conflicts = append(conflicts, rec) // conflicts shall be recreated
			}
		}
	}
	for _, record := range conflicts {
		err := dnsad.DelRecord(record)
		if err != nil {
			updates <- &executor.ExecutorUpdate{CurrentStep: rpc.STEP_PUSH_SUMMARY, StepMessage: "Failed to delete conflict " + record.Domain + ": " + err.Error()}
		}
	}
	for host, ip := range dnsrecords {
		// Ignore raw IP's
		if strings.Contains(host, "in-addr") {
			continue
		}
		ipaddr := net.ParseIP(ip)
		err := dnsad.AddRecord(dnsadapters.DNSRecord{Domain: host, Addr: ipaddr, Type: "A"})
		if err != nil {
			updates <- &executor.ExecutorUpdate{CurrentStep: rpc.STEP_PUSH_SUMMARY, StepMessage: "Failed to add " + host + "\tIN\tA\t" + ip + ": " + err.Error()}
			return ctx
		}
		updates <- &executor.ExecutorUpdate{CurrentStep: rpc.STEP_PUSH_SUMMARY, StepMessage: "Added " + host + "\tIN\tA\t" + ip}
	}
	err = UpdatePlaybookDB(s.playbookDB, curpb)
	s.UpdateUpdaterTable()
	if err != nil {
		updates <- &executor.ExecutorUpdate{CurrentStep: rpc.STEP_ERROR, StepMessage: "Failed updating playbook in db: " + err.Error()}
	}
	dnsad.CommitRecords()
	return ctx

}
