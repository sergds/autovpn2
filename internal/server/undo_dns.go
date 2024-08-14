package server

import (
	"context"

	dnsadapters "github.com/sergds/autovpn2/internal/adapters/dns"
	"github.com/sergds/autovpn2/internal/executor"
	"github.com/sergds/autovpn2/internal/playbook"
	"github.com/sergds/autovpn2/internal/rpc"
)

func (s *AutoVPNServer) StepUndoDNS(updates chan *executor.ExecutorUpdate, ctx context.Context) context.Context {
	curpb := ctx.Value("playbook").(*playbook.Playbook)

	var dnsad dnsadapters.DNSAdapter = dnsadapters.NewDNSAdapter(curpb.Adapters.Dns)
	if dnsad == nil {
		updates <- &executor.ExecutorUpdate{CurrentStep: rpc.STEP_ERROR, StepMessage: "Failed to create dns adapter " + curpb.Adapters.Dns}
		return ctx
	}
	err := dnsad.Authenticate(curpb.Adapterconfig.Dns)
	if err == nil {
		updates <- &executor.ExecutorUpdate{CurrentStep: rpc.STEP_PUSH_SUMMARY, StepMessage: "Authenticated!"}
	} else {
		updates <- &executor.ExecutorUpdate{CurrentStep: rpc.STEP_ERROR, StepMessage: "Failed to authenticate on " + curpb.Adapters.Dns + ". Check credentials! " + err.Error()}
		return ctx
	}
	var records []dnsadapters.DNSRecord = make([]dnsadapters.DNSRecord, 0)
	recs := dnsad.GetRecords("A")
	for _, rec := range recs {
		for _, domain := range curpb.Hosts {
			if rec.Domain == domain {
				records = append(records, rec) // delete records that intersect with the applied ones.
			}
		}
	}
	for _, record := range records {
		err := dnsad.DelRecord(record)
		if err != nil {
			updates <- &executor.ExecutorUpdate{CurrentStep: rpc.STEP_PUSH_SUMMARY, StepMessage: "Failed to delete " + record.Domain + ": " + err.Error()}
		}
		updates <- &executor.ExecutorUpdate{CurrentStep: rpc.STEP_PUSH_SUMMARY, StepMessage: "Deleted " + record.Domain}
	}
	dnsad.CommitRecords()
	return ctx
}
