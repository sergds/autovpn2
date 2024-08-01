package server

import (
	"context"
	"encoding/gob"
	"errors"
	"fmt"
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
	bolt "go.etcd.io/bbolt"
	"google.golang.org/grpc"
)

var clear string = "\t\t\t\t\t\t"

type AutoVPNServer struct {
	pb.UnimplementedAutoVPNServer
	playbookDB *bolt.DB
}

func GetAllPlaybooksFromDB(db *bolt.DB) map[string]*playbook.Playbook {
	var playbooks map[string]*playbook.Playbook = make(map[string]*playbook.Playbook)
	db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("playbook_obj"))

		c := b.Cursor()

		for k, v := c.First(); k != nil; k, v = c.Next() {
			var pb *playbook.Playbook = &playbook.Playbook{}
			err := gob.NewDecoder(strings.NewReader(string(v))).Decode(pb)
			if err != nil {
				log.Println(err)
				continue
			}
			playbooks[string(k)] = pb
		}
		return nil
	})
	return playbooks
}

func DeletePlaybookDB(db *bolt.DB, pb *playbook.Playbook) error {
	err := db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("playbook_obj"))
		b.Delete([]byte(pb.Name))
		return nil
	})
	return err
}

func UpdatePlaybookDB(db *bolt.DB, pb *playbook.Playbook) error {
	err := db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("playbook_obj"))
		pbgob := &strings.Builder{}
		err := gob.NewEncoder(pbgob).Encode(pb)
		if err != nil {
			return errors.New("db transaction failed: " + err.Error())
		}
		b.Put([]byte(pb.Name), []byte(pbgob.String()))
		return nil
	})
	return err
}

func (s *AutoVPNServer) Apply(in *pb.ApplyRequest, ss pb.AutoVPN_ApplyServer) error {
	curpb, err := playbook.Parse(in.GetPlaybook())
	if err != nil {
		st := "Failed to parse playbook! " + err.Error()
		ss.Send(&pb.ApplyResponse{Status: pb.STATUS_ERROR, Statustext: &st})
		return nil
	}
	pbooks := GetAllPlaybooksFromDB(s.playbookDB)
	for pname, pbook := range pbooks {
		if curpb.Name == pname && pbook.GetInstallState() {
			s.reportStatus(ss, "There is already a playbook named "+curpb.Name+"! Undo it first!", pb.STATUS_ERROR)
			return nil
		}
	}
	if !curpb.Lock("Apply") {
		s.reportStatus(ss, "Unexpected lock on fresh playbook! (reason: "+curpb.GetLockReason()+")", pb.STATUS_ERROR)
		return nil
	}
	err = UpdatePlaybookDB(s.playbookDB, curpb)
	if err != nil {
		s.reportStatus(ss, "Failed adding playbook to db: "+err.Error(), pb.STATUS_ERROR)
		return nil
	}
	ss.Send(&pb.ApplyResponse{Status: pb.STATUS_FETCHIP})
	dnsrecords := s.FetchIPs(curpb, ss)
	if curpb.Custom != nil {
		for h, ip := range curpb.Custom {
			dnsrecords[h] = ip
		}
	}
	curpb.PlaybookAddrs = dnsrecords
	err = UpdatePlaybookDB(s.playbookDB, curpb)
	if err != nil {
		s.reportStatus(ss, "Failed updating playbook in db: "+err.Error(), pb.STATUS_ERROR)
		return nil
	}
	s.reportStatus(ss, "Authenticating with DNS Adapter...", pb.STATUS_DNS)
	notok, err := s.ApplyDNS(curpb, ss, dnsrecords)
	if notok {
		return err
	}
	// Since the first change has been commited, the playbook is now deemed "installed"
	err = UpdatePlaybookDB(s.playbookDB, curpb)
	if err != nil {
		s.reportStatus(ss, "Failed updating playbook in db: "+err.Error(), pb.STATUS_ERROR)
		return nil
	}
	s.reportStatus(ss, "Authenticating with "+curpb.Adapters.Routes+" route adapter...", pb.STATUS_ROUTES)
	failedroutes, notok, returnValue := s.ApplyRoutes(curpb, ss, dnsrecords)
	if notok {
		return returnValue
	}
	curpb.Unlock()
	err = UpdatePlaybookDB(s.playbookDB, curpb)
	if err != nil {
		s.reportStatus(ss, "Failed updating playbook in db: "+err.Error(), pb.STATUS_ERROR)
		return nil
	}
	s.reportStatus(ss, "Finished", pb.STATUS_PUSH_SUMMARY)

	if len(failedroutes) != 0 {
		st := "Following Routes failed to add: " + strings.Join(failedroutes, ", ") + ". Manual intervention is likely needed"
		ss.Send(&pb.ApplyResponse{Status: pb.STATUS_PUSH_SUMMARY, Statustext: &st})
	}
	return nil
}

func (s *AutoVPNServer) ApplyRoutes(curpb *playbook.Playbook, ss pb.AutoVPN_ApplyServer, dnsrecords map[string]string) ([]string, bool, error) {
	var routead routes.RouteAdapter = routes.NewRouteAdapter(curpb.Adapters.Routes)
	failedroutes := make([]string, 0)
	if routead == nil {
		s.reportStatus(ss, "Failed to create route adapter "+curpb.Adapters.Routes, pb.STATUS_ERROR)
		time.Sleep(time.Millisecond * 2000)
		return nil, true, nil
	}
	err := routead.Authenticate(curpb.Adapterconfig.Routes["creds"], curpb.Adapterconfig.Routes["endpoint"])
	if err == nil {
		s.reportStatus(ss, "Authenticated!", pb.STATUS_ROUTES)
		time.Sleep(time.Millisecond * 2000)
	} else {
		s.reportStatus(ss, "Failed to authenticate on "+curpb.Adapters.Routes+": "+err.Error(), pb.STATUS_ERROR)
		time.Sleep(time.Millisecond * 2000)
		return nil, true, nil
	}
	cur_routes, err := routead.GetRoutes()
	if err != nil {
		s.reportStatus(ss, "Failed to get routes from "+curpb.Adapters.Routes+": "+err.Error(), pb.STATUS_ERROR)
		time.Sleep(time.Millisecond * 2000)
		return nil, true, nil
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
		s.reportStatus(ss, "There are conflicts! The conflicting routes will be recreated!", pb.STATUS_ROUTES)
		time.Sleep(time.Millisecond * 2000)
		s.reportStatus(ss, "Removing conflicts...", pb.STATUS_ROUTES)
		for _, r := range route_conflicts {
			err := routead.DelRoute(*r)
			if err != nil {
				s.reportStatus(ss, "Failed to delete a route "+r.Destination+": "+err.Error(), pb.STATUS_ERROR)
				return nil, true, nil
			}
		}
	}
	for h, ip := range dnsrecords {
		err := routead.AddRoute(routes.Route{Destination: ip, Gateway: "0.0.0.0", Interface: curpb.Interface, Comment: "[AutoVPN2] Playbook: " + curpb.Name + " Host: " + h})
		if err != nil {
			s.reportStatus(ss, "Failed to add a route "+ip+": "+err.Error(), pb.STATUS_PUSH_SUMMARY)
			failedroutes = append(failedroutes, ip)
			continue
		}
		s.reportStatus(ss, "Routed "+ip+"\t->\t"+curpb.Interface, pb.STATUS_ROUTES)
	}
	s.reportStatus(ss, "Saving changes", pb.STATUS_NOTIFY)
	routead.SaveConfig()
	return failedroutes, false, nil
}

func (s *AutoVPNServer) ApplyDNS(curpb *playbook.Playbook, ss pb.AutoVPN_ApplyServer, dnsrecords map[string]string) (bool, error) {
	var dnsad dnsadapters.DNSAdapter = dnsadapters.NewDNSAdapter(curpb.Adapters.Dns)
	failednames := make([]string, 0)
	if err := dnsad.Authenticate(curpb.Adapterconfig.Dns["creds"], curpb.Adapterconfig.Dns["endpoint"]); err == nil {
		s.reportStatus(ss, "Authenticated!", pb.STATUS_DNS)
		time.Sleep(1 * time.Second)
	} else {
		s.reportStatus(ss, "Unauthorized!", pb.STATUS_ERROR)
		time.Sleep(1 * time.Second)
		return true, nil
	}
	for host, ip := range dnsrecords {
		ipaddr := net.ParseIP(ip)
		err := dnsad.AddRecord(dnsadapters.DNSRecord{Domain: host, Addr: ipaddr, Type: "A"})
		if err != nil {
			s.reportStatus(ss, "Failed to add "+host+"\tIN\tA\t"+ip+": "+err.Error(), pb.STATUS_ERROR)
			failednames = append(failednames, host)
			return true, nil
		}
		curpb.SetInstallState(true)
		st := "Added " + host + "\tIN\tA\t" + ip
		ss.Send(&pb.ApplyResponse{Status: pb.STATUS_DNS, Statustext: &st})
	}
	dnsad.CommitRecords()
	if len(failednames) != 0 {
		st := "Following DNS records failed to add: " + strings.Join(failednames, ", ") + ". Manual intervention is likely needed"
		ss.Send(&pb.ApplyResponse{Status: pb.STATUS_PUSH_SUMMARY, Statustext: &st})
	}
	return false, nil
}

func (s *AutoVPNServer) FetchIPs(curpb *playbook.Playbook, ss pb.AutoVPN_ApplyServer) map[string]string {
	var dnsrecords map[string]string = make(map[string]string)
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
			s.reportStatus(ss, "Failed to resolve domain "+host+"! "+err.Error(), pb.STATUS_PUSH_SUMMARY)
			continue
		}
		if answ != "" {
			dnsrecords[host] = answ
			s.reportStatus(ss, "Resolved "+host+"\tIN\tA\t"+answ, pb.STATUS_FETCHIP)
		} else {
			s.reportStatus(ss, "Failed getting INET Address of "+host+"!", pb.STATUS_PUSH_SUMMARY)
			continue
		}
	}
	return dnsrecords
}

func (*AutoVPNServer) reportStatus(ss pb.AutoVPN_ApplyServer, msg string, status int32) {
	st := msg
	ss.Send(&pb.ApplyResponse{Status: status, Statustext: &st})
}

func (*AutoVPNServer) reportStatusUndo(ss pb.AutoVPN_UndoServer, msg string, status int32) {
	st := msg
	ss.Send(&pb.UndoResponse{Status: status, Statustext: &st})
}

func (s *AutoVPNServer) List(ctx context.Context, in *pb.ListRequest) (*pb.ListResponse, error) {
	pbooks := GetAllPlaybooksFromDB(s.playbookDB)
	var pbnames []string = make([]string, 0)
	for pbname, _ := range pbooks {
		pbnames = append(pbnames, pbname)
	}
	return &pb.ListResponse{Playbooks: pbnames}, nil
}

func (s *AutoVPNServer) Undo(in *pb.UndoRequest, ss pb.AutoVPN_UndoServer) error {
	var ok bool = false
	var wasinstalled bool = false
	var curpb *playbook.Playbook = nil
	pbooks := GetAllPlaybooksFromDB(s.playbookDB)

	for _, pbook := range pbooks {
		if pbook.Name == in.Playbookname {
			ok = true
			curpb = pbook
			if pbook.GetInstallState() {
				wasinstalled = true
			}
		}
	}
	if !ok {
		s.reportStatusUndo(ss, "No such playbook "+in.Playbookname+" installed!", pb.UNDO_STATUS_ERROR)
		return nil
	}
	if !wasinstalled {
		s.reportStatusUndo(ss, "Such playbook exists, but not installed! Removing!", pb.UNDO_STATUS_ERROR)
		DeletePlaybookDB(s.playbookDB, curpb)
		return nil
	}
	if !curpb.Lock("Undo") {
		s.reportStatusUndo(ss, "Playbook is being processed at the moment (reason: "+curpb.GetLockReason()+")!", pb.UNDO_STATUS_ERROR)
		return nil
	}
	ss.Send(&pb.UndoResponse{Status: pb.UNDO_STATUS_DNS})
	shouldReturn, returnValue := s.UndoDNS(curpb, ss, in)
	if shouldReturn {
		return returnValue
	}
	ss.Send(&pb.UndoResponse{Status: pb.UNDO_STATUS_ROUTES})
	// Try getting addrs from route addresses.
	failedroutes, shouldReturn1, returnValue1 := s.UndoRoutes(curpb, ss)
	if shouldReturn1 {
		return returnValue1
	}
	s.reportStatusUndo(ss, "Finished", pb.UNDO_STATUS_PUSH_SUMMARY)
	if len(failedroutes) != 0 {
		s.reportStatusUndo(ss, "Following routes failed to delete: "+strings.Join(failedroutes, ", ")+". Manual intervention is likely needed", pb.UNDO_STATUS_PUSH_SUMMARY)
	}
	DeletePlaybookDB(s.playbookDB, curpb)
	curpb.Unlock()
	return nil
}

func (*AutoVPNServer) UndoRoutes(curpb *playbook.Playbook, ss pb.AutoVPN_UndoServer) ([]string, bool, error) {
	st := "Authenticating with " + curpb.Adapters.Routes + " route adapter..."
	ss.Send(&pb.UndoResponse{Status: pb.UNDO_STATUS_ROUTES, Statustext: &st})
	var routead routes.RouteAdapter = routes.NewRouteAdapter(curpb.Adapters.Routes)
	if routead == nil {
		st := "Failed to create route adapter " + curpb.Adapters.Routes
		ss.Send(&pb.UndoResponse{Status: pb.UNDO_STATUS_ERROR, Statustext: &st})
		time.Sleep(time.Millisecond * 2000)
		return nil, true, nil
	}
	err := routead.Authenticate(curpb.Adapterconfig.Routes["creds"], curpb.Adapterconfig.Routes["endpoint"])
	failedroutes := make([]string, 0)
	if err == nil {
		st := "Authenticated!"
		ss.Send(&pb.UndoResponse{Status: pb.UNDO_STATUS_ROUTES, Statustext: &st})
		time.Sleep(time.Millisecond * 2000)
	} else {
		st := "Failed to authenticate on " + curpb.Adapters.Routes + ": " + err.Error()
		ss.Send(&pb.UndoResponse{Status: pb.UNDO_STATUS_ERROR, Statustext: &st})
		time.Sleep(time.Millisecond * 2000)
		return nil, true, nil
	}

	st = "Trying to get addresses from route addresses"
	ss.Send(&pb.UndoResponse{Status: pb.UNDO_STATUS_ROUTES, Statustext: &st})
	time.Sleep(time.Millisecond * 500)
	var addrs []string = make([]string, 0)
	cur_routes, err := routead.GetRoutes()
	if err != nil {
		for _, ip := range curpb.PlaybookAddrs {
			addrs = append(addrs, ip)
		}
		st := "Falling back to address cold storage!"
		ss.Send(&pb.UndoResponse{Status: pb.UNDO_STATUS_ROUTES, Statustext: &st})
		time.Sleep(time.Millisecond * 1500)
	} else {
		st := "Retrieved needed addresses from router adapter!"
		ss.Send(&pb.UndoResponse{Status: pb.UNDO_STATUS_ROUTES, Statustext: &st})
		time.Sleep(time.Millisecond * 1500)
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
			failedroutes = append(failedroutes, ip)
		}
		st := "Unrouted " + ip
		ss.Send(&pb.UndoResponse{Status: pb.UNDO_STATUS_ROUTES, Statustext: &st})
	}
	routead.SaveConfig()

	return failedroutes, false, nil
}

func (s *AutoVPNServer) UndoDNS(curpb *playbook.Playbook, ss pb.AutoVPN_UndoServer, in *pb.UndoRequest) (bool, error) {
	var dnsad dnsadapters.DNSAdapter = dnsadapters.NewDNSAdapter(curpb.Adapters.Dns)
	if dnsad == nil {
		st := "Failed to create dns adapter " + curpb.Adapters.Dns
		ss.Send(&pb.UndoResponse{Status: pb.UNDO_STATUS_ERROR, Statustext: &st})
		time.Sleep(time.Millisecond * 2000)
		return true, nil
	}
	err := dnsad.Authenticate(curpb.Adapterconfig.Dns["creds"], curpb.Adapterconfig.Dns["endpoint"])
	failednames := make([]string, 0)
	if err == nil {
		st := "Authenticated!"
		ss.Send(&pb.UndoResponse{Status: pb.UNDO_STATUS_DNS, Statustext: &st})
		time.Sleep(time.Millisecond * 2000)
	} else {
		st := "Failed to authenticate on " + curpb.Adapters.Dns + ". Check credentials! " + err.Error()
		ss.Send(&pb.UndoResponse{Status: pb.UNDO_STATUS_ERROR, Statustext: &st})
		time.Sleep(time.Millisecond * 2000)
		return true, nil
	}
	var records []dnsadapters.DNSRecord = make([]dnsadapters.DNSRecord, 0)
	recs := dnsad.GetRecords("A")
	for _, rec := range recs {
		for _, domain := range curpb.Hosts {
			if rec.Domain == domain {
				records = append(records, rec)
			}
		}
	}
	for _, record := range records {
		err := dnsad.DelRecord(record)
		if err != nil {
			st := "Failed to delete " + record.Domain + ": " + err.Error()
			failednames = append(failednames, record.Domain)
			ss.Send(&pb.UndoResponse{Status: pb.UNDO_STATUS_DNS, Statustext: &st})
			time.Sleep(1 * time.Second)
		}
		st := "Deleted " + record.Domain
		ss.Send(&pb.UndoResponse{Status: pb.UNDO_STATUS_DNS, Statustext: &st})
	}
	dnsad.CommitRecords()
	if len(failednames) != 0 {
		st := "Following DNS records failed to delete: " + strings.Join(failednames, ", ") + ". Manual intervention is likely needed"
		ss.Send(&pb.UndoResponse{Status: pb.UNDO_STATUS_PUSH_SUMMARY, Statustext: &st})
	}
	return false, nil
}

func ServerMain() {
	lis, err := net.Listen("tcp", "0.0.0.0:15328")
	if err != nil {
		log.Fatalln(err.Error())
	}
	s := grpc.NewServer()
	var dbpath string = os.Getenv("AVPN2_BOLTPATH")
	if dbpath != "" {
		dbpath += string(os.PathSeparator)
	}
	pbdb, err := bolt.Open(dbpath+"avpn2_playbooks.db", 0666, &bolt.Options{})
	if err != nil {
		log.Println("failed to open pbdb: " + err.Error())
		os.Exit(1)
	}
	err = pbdb.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte("playbook_obj"))
		if err != nil {
			return fmt.Errorf("create bucket: %s", err)
		}
		return nil
	})
	if err != nil {
		log.Fatalf("failed preparing pbdb: %s", err)
	}
	pb.RegisterAutoVPNServer(s, &AutoVPNServer{playbookDB: pbdb})
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
