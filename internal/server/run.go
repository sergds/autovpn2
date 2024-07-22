package server

import (
	"context"
	"log"
	"net"
	"os"
	"strings"
	"time"

	"github.com/grandcat/zeroconf"
	"github.com/likexian/doh"
	"github.com/likexian/doh/dns"
	dnsadapters "github.com/sergds/autovpn2/internal/adapters/dns"
	"github.com/sergds/autovpn2/internal/adapters/routes"
	"github.com/sergds/autovpn2/internal/playbook"
	pb "github.com/sergds/autovpn2/internal/rpc"
	"google.golang.org/grpc"
	"gopkg.in/yaml.v3"
)

var clear string = "\t\t\t\t\t\t"

type AutoVPNServer struct {
	pb.UnimplementedAutoVPNServer
	playbooksInstalled map[*playbook.Playbook]bool // bool indicates if playbook was successfully applied.
	playbookAddrs      map[string]map[string]string
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
	s.playbooksInstalled[playbook] = false
	ss.Send(&pb.ApplyResponse{Status: pb.STATUS_FETCHIP})
	var dnsrecords map[string]string = make(map[string]string) // host:ip
	for _, host := range playbook.Hosts {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		c := doh.Use(doh.CloudflareProvider)
		resp, err := c.Query(ctx, dns.Domain(host), dns.TypeA)
		answ := ""
		for _, a := range resp.Answer {
			if a.Type == 1 { // 1 -- A
				answ = a.Data
			}
		}
		if err != nil {
			st := "Failed to resolve domain " + host + "! " + err.Error()
			ss.Send(&pb.ApplyResponse{Status: pb.STATUS_ERROR, Statustext: &st})
			return nil
		}
		dnsrecords[host] = answ
		st := "Resolved " + host + "\tIN\tA\t" + answ
		ss.Send(&pb.ApplyResponse{Status: pb.STATUS_FETCHIP, Statustext: &st})
	}
	if playbook.Custom != nil {
		for h, ip := range playbook.Custom {
			dnsrecords[h] = ip
		}
	}
	s.playbookAddrs[playbook.Name] = dnsrecords
	st := "Authenticating with DNS Adapter..."
	ss.Send(&pb.ApplyResponse{Status: pb.STATUS_DNS, Statustext: &st})
	var dnsad dnsadapters.DNSAdapter = dnsadapters.NewDNSAdapter(playbook.Adapters.Dns)
	if err := dnsad.Authenticate(playbook.Adapterconfig.Dns["creds"], playbook.Adapterconfig.Dns["endpoint"]); err == nil {
		st := "Authenticated!"
		ss.Send(&pb.ApplyResponse{Status: pb.STATUS_DNS, Statustext: &st})
		time.Sleep(1 * time.Second)
	} else {
		st := "Unauthorized! " + err.Error()
		ss.Send(&pb.ApplyResponse{Status: pb.STATUS_ERROR, Statustext: &st})
		time.Sleep(1 * time.Second)
		return nil
	}
	for host, ip := range dnsrecords {
		ipaddr := net.ParseIP(ip)
		err := dnsad.AddRecord(dnsadapters.DNSRecord{Domain: host, Addr: ipaddr, Type: "A"})
		if err != nil {
			st := "Failed to add " + host + "\tIN\tA\t" + ip + ". " + err.Error()
			ss.Send(&pb.ApplyResponse{Status: pb.STATUS_ERROR, Statustext: &st})
			return nil
		}
		s.playbooksInstalled[playbook] = true // Since the first change has been commited, the playbook is now deemed "installed"
		st := "Added " + host + "\tIN\tA\t" + ip
		ss.Send(&pb.ApplyResponse{Status: pb.STATUS_DNS, Statustext: &st})
	}
	dnsad.CommitRecords()
	st = "Authenticating with " + playbook.Adapters.Routes + " route adapter..."
	ss.Send(&pb.ApplyResponse{Status: pb.STATUS_ROUTES, Statustext: &st})
	var routead routes.RouteAdapter = routes.NewRouteAdapter(playbook.Adapters.Routes)
	if routead == nil {
		st := "Failed to create route adapter " + playbook.Adapters.Routes
		ss.Send(&pb.ApplyResponse{Status: pb.STATUS_ERROR, Statustext: &st})
		time.Sleep(time.Millisecond * 2000)
		return nil
	}
	ok := routead.Authenticate(playbook.Adapterconfig.Routes["creds"], playbook.Adapterconfig.Routes["endpoint"])
	if ok {
		st := "Authenticated!"
		ss.Send(&pb.ApplyResponse{Status: pb.STATUS_ROUTES, Statustext: &st})
		time.Sleep(time.Millisecond * 2000)
	} else {
		st := "Failed to authenticate on " + playbook.Adapters.Routes + ". Check credentials!"
		ss.Send(&pb.ApplyResponse{Status: pb.STATUS_ERROR, Statustext: &st})
		time.Sleep(time.Millisecond * 2000)
		return nil
	}
	cur_routes := routead.GetRoutes()
	route_conflicts := make([]routes.Route, 0)
	for _, r := range cur_routes {
		ip := strings.Split(r.Destination, "/")[0] // strip mask notation
		for _, newip := range dnsrecords {
			if ip == newip && r.Interface == playbook.Interface {
				route_conflicts = append(route_conflicts, r)
			}
		}
	}
	if len(route_conflicts) != 0 {
		st := "There are conflicts! The conflicting routes will be recreated!"
		ss.Send(&pb.ApplyResponse{Status: pb.STATUS_ROUTES, Statustext: &st})
		time.Sleep(time.Millisecond * 2000)
		st = "Removing conflicts..."
		ss.Send(&pb.ApplyResponse{Status: pb.STATUS_ROUTES, Statustext: &st})
		for _, r := range route_conflicts {
			ok := routead.DelRoute(r)
			if !ok {
				st := "Failed to delete a route! " + r.Destination
				ss.Send(&pb.ApplyResponse{Status: pb.STATUS_ERROR, Statustext: &st})
				return nil
			}
		}
	}
	for h, ip := range dnsrecords {
		ok := routead.AddRoute(routes.Route{Destination: ip, Gateway: "0.0.0.0", Interface: playbook.Interface}, "[AutoVPN2] Playbook: "+playbook.Name+" Host: "+h)
		if !ok {
			st := "Failed to add a route! " + ip
			ss.Send(&pb.ApplyResponse{Status: pb.STATUS_ERROR, Statustext: &st})
			return nil
		}
		st := "Routed " + ip + "\t->\t" + playbook.Interface
		ss.Send(&pb.ApplyResponse{Status: pb.STATUS_ROUTES, Statustext: &st})
	}
	st = "Saving changes"
	ss.Send(&pb.ApplyResponse{Status: pb.STATUS_NOTIFY, Statustext: &st})
	routead.SaveConfig()
	return nil
}

func (s *AutoVPNServer) List(ctx context.Context, in *pb.ListRequest) (*pb.ListResponse, error) {
	var pbnames []string = make([]string, 0)
	for pbook := range s.playbooksInstalled {
		pbnames = append(pbnames, pbook.Name)
	}
	return &pb.ListResponse{Playbooks: pbnames}, nil
}

func (s *AutoVPNServer) Undo(in *pb.UndoRequest, ss pb.AutoVPN_UndoServer) error {
	var ok bool = false
	var wasinstalled bool = false
	var curpb *playbook.Playbook = nil
	for pbook, inst := range s.playbooksInstalled {
		if pbook.Name == in.Playbookname {
			ok = true
			curpb = pbook
			if inst {
				wasinstalled = true
			}
		}
	}
	if !ok {
		st := "No such playbook installed!"
		ss.Send(&pb.UndoResponse{Status: pb.UNDO_STATUS_ERROR, Statustext: &st})
		return nil
	}
	if !wasinstalled {
		st := "No such playbook installed!"
		ss.Send(&pb.UndoResponse{Status: pb.UNDO_STATUS_ERROR, Statustext: &st})
		return nil
	}
	ss.Send(&pb.UndoResponse{Status: pb.UNDO_STATUS_DNS})
	var dnsad dnsadapters.DNSAdapter = dnsadapters.NewDNSAdapter(curpb.Adapters.Dns)
	if dnsad == nil {
		st := "Failed to create dns adapter " + curpb.Adapters.Dns
		ss.Send(&pb.UndoResponse{Status: pb.UNDO_STATUS_ERROR, Statustext: &st})
		time.Sleep(time.Millisecond * 2000)
		return nil
	}
	err := dnsad.Authenticate(curpb.Adapterconfig.Dns["creds"], curpb.Adapterconfig.Dns["endpoint"])
	if err == nil {
		st := "Authenticated!"
		ss.Send(&pb.UndoResponse{Status: pb.UNDO_STATUS_ROUTES, Statustext: &st})
		time.Sleep(time.Millisecond * 2000)
	} else {
		st := "Failed to authenticate on " + curpb.Adapters.Dns + ". Check credentials! " + err.Error()
		ss.Send(&pb.UndoResponse{Status: pb.UNDO_STATUS_ERROR, Statustext: &st})
		time.Sleep(time.Millisecond * 2000)
		return nil
	}
	for _, domain := range curpb.Hosts {
		domaddr := net.ParseIP(s.playbookAddrs[in.Playbookname][domain])
		err := dnsad.DelRecord(dnsadapters.DNSRecord{Domain: domain, Addr: domaddr})
		if err != nil {
			st := "Failed to delete " + domain + ": " + err.Error()
			ss.Send(&pb.UndoResponse{Status: pb.UNDO_STATUS_DNS, Statustext: &st})
			time.Sleep(1 * time.Second)
		}
		st := "Deleted " + domain
		ss.Send(&pb.UndoResponse{Status: pb.UNDO_STATUS_DNS, Statustext: &st})
	}
	dnsad.CommitRecords()
	ss.Send(&pb.UndoResponse{Status: pb.UNDO_STATUS_ROUTES})
	st := "Authenticating with " + curpb.Adapters.Routes + " route adapter..."
	ss.Send(&pb.UndoResponse{Status: pb.UNDO_STATUS_ROUTES, Statustext: &st})
	var routead routes.RouteAdapter = routes.NewRouteAdapter(curpb.Adapters.Routes)
	if routead == nil {
		st := "Failed to create route adapter " + curpb.Adapters.Routes
		ss.Send(&pb.UndoResponse{Status: pb.UNDO_STATUS_ERROR, Statustext: &st})
		time.Sleep(time.Millisecond * 2000)
		return nil
	}
	adok := routead.Authenticate(curpb.Adapterconfig.Routes["creds"], curpb.Adapterconfig.Routes["endpoint"])
	if adok {
		st := "Authenticated!"
		ss.Send(&pb.UndoResponse{Status: pb.UNDO_STATUS_ROUTES, Statustext: &st})
		time.Sleep(time.Millisecond * 2000)
	} else {
		st := "Failed to authenticate on " + curpb.Adapters.Routes + ". Check credentials!"
		ss.Send(&pb.UndoResponse{Status: pb.UNDO_STATUS_ERROR, Statustext: &st})
		time.Sleep(time.Millisecond * 2000)
		return nil
	}
	for _, ip := range s.playbookAddrs[curpb.Name] {
		routead.DelRoute(routes.Route{Destination: ip, Gateway: "0.0.0.0", Interface: curpb.Interface})
		st := "Unrouted " + ip
		ss.Send(&pb.UndoResponse{Status: pb.UNDO_STATUS_ROUTES, Statustext: &st})
	}
	st = "Finished"
	ss.Send(&pb.UndoResponse{Status: pb.UNDO_STATUS_NOTIFY, Statustext: &st})
	delete(s.playbookAddrs, curpb.Name)
	delete(s.playbooksInstalled, curpb)
	return nil
}

func ServerMain() {
	lis, err := net.Listen("tcp", "0.0.0.0:15328")
	if err != nil {
		log.Fatalln(err.Error())
	}
	s := grpc.NewServer()
	pb.RegisterAutoVPNServer(s, &AutoVPNServer{playbooksInstalled: make(map[*playbook.Playbook]bool), playbookAddrs: make(map[string]map[string]string)})
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
