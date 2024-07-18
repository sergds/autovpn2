package server

import (
	"context"
	"log"
	"net"
	"os"
	"time"

	"github.com/grandcat/zeroconf"
	"github.com/likexian/doh"
	"github.com/likexian/doh/dns"
	"github.com/sergds/autovpn2/internal/playbook"
	pb "github.com/sergds/autovpn2/internal/rpc"
	"google.golang.org/grpc"
	"gopkg.in/yaml.v3"
)

var clear string = "\t\t\t\t\t\t"

type AutoVPNServer struct {
	pb.UnimplementedAutoVPNServer
	playbooksInstalled map[*playbook.Playbook]bool // bool indicates if playbook was successfully applied.
}

func (s *AutoVPNServer) Apply(in *pb.ApplyRequest, ss pb.AutoVPN_ApplyServer) error {
	play := in.GetPlaybook()
	playbook := &playbook.Playbook{}
	err := yaml.Unmarshal([]byte(play), playbook)
	for playb, ok := range s.playbooksInstalled {
		if playbook.Name == playb.Name && ok {
			st := "There is already a playbook named " + playbook.Name + "! Undo it first!"
			ss.Send(&pb.ApplyResponse{Status: pb.STATUS_ERROR, Statustext: &st})
			return nil
		}
	}
	if err != nil {
		st := "Failed to parse playbook! " + err.Error()
		ss.Send(&pb.ApplyResponse{Status: pb.STATUS_ERROR, Statustext: &st})
		return nil
	}
	ss.Send(&pb.ApplyResponse{Status: pb.STATUS_FETCHIP})
	var dnsrecords map[string]string = make(map[string]string) // host:ip
	for _, host := range playbook.Hosts {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		c := doh.Use(doh.CloudflareProvider)
		resp, err := c.Query(ctx, dns.Domain(host), dns.TypeA)
		if err != nil {
			st := "Failed to resolve domain " + host + "! " + err.Error()
			ss.Send(&pb.ApplyResponse{Status: pb.STATUS_ERROR, Statustext: &st})
			return nil
		}
		dnsrecords[host] = resp.Answer[0].Data
		st := "Resolved " + host + "\tIN\tA\t" + resp.Answer[0].Data
		ss.Send(&pb.ApplyResponse{Status: pb.STATUS_FETCHIP, Statustext: &st})
	}
	s.playbooksInstalled[playbook] = false
	ss.Send(&pb.ApplyResponse{Status: pb.STATUS_DNS, Statustext: &clear})
	for host, ip := range dnsrecords {
		time.Sleep(time.Second * 1)
		st := "Added " + host + "\tIN\tA\t" + ip
		ss.Send(&pb.ApplyResponse{Status: pb.STATUS_DNS, Statustext: &st})
	}
	ss.Send(&pb.ApplyResponse{Status: pb.STATUS_ROUTES, Statustext: &clear})
	for _, ip := range dnsrecords {
		time.Sleep(time.Millisecond * 800)
		st := "Routed " + ip + "\t->\t" + playbook.Interface
		ss.Send(&pb.ApplyResponse{Status: pb.STATUS_ROUTES, Statustext: &st})
	}
	//s.playbooksInstalled[playbook] = true
	st := "Finished"
	ss.Send(&pb.ApplyResponse{Status: pb.STATUS_NOTIFY, Statustext: &st})
	return nil
}

func (s *AutoVPNServer) List(ctx context.Context, in *pb.ListRequest) (*pb.ListResponse, error) {
	return &pb.ListResponse{Playbooks: []string{"todo"}}, nil
}

func (s *AutoVPNServer) Undo(in *pb.UndoRequest, ss pb.AutoVPN_UndoServer) error {
	return nil
}

func ServerMain() {
	lis, err := net.Listen("tcp", "0.0.0.0:15328")
	if err != nil {
		log.Fatalln(err.Error())
	}
	s := grpc.NewServer()
	pb.RegisterAutoVPNServer(s, &AutoVPNServer{playbooksInstalled: make(map[*playbook.Playbook]bool)})
	host, _ := os.Hostname()
	server, err := zeroconf.Register("AutoVPN Server @ "+host, "_autovpn._tcp", "local.", 15328, []string{"txtv=0", "host=" + host}, nil)
	defer server.Shutdown()
	if err != nil {
		log.Fatalln("Failed to initialize mDNS:", err.Error())
	}

	log.Printf("autovpn server running @ %s", lis.Addr().String())
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
