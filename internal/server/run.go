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
	"github.com/sergds/autovpn2/internal/executor"
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

func (*AutoVPNServer) reportStatus(ss pb.AutoVPN_ExecuteTaskServer, state string, msg string) {
	st := "[" + state + "] " + pb.DescribeState(state)
	ss.Send(&pb.ExecuteUpdate{Statecode: state, Statetext: &st, Opdesc: &msg})
}

func (s *AutoVPNServer) ExecuteTask(in *pb.ExecuteRequest, ss pb.AutoVPN_ExecuteTaskServer) error {
	s.reportStatus(ss, pb.STEP_NOTIFY, "Building Executor")
	var ex *executor.Executor = executor.NewExecutor()
	switch in.Operation { // Build Executor
	case pb.TASK_LIST:
		ex.AddStep(executor.NewStep(pb.STEP_LIST, s.StepList))
	case pb.TASK_APPLY:
		{
			ex.AddStep(executor.NewStep("prep_ctx", func(updates chan *executor.ExecutorUpdate, ctx context.Context) context.Context {
				curpb, err := playbook.Parse(in.Argv[0])
				if err != nil {
					updates <- &executor.ExecutorUpdate{CurrentStep: pb.STEP_ERROR, StepMessage: "Failed to parse playbook!"}
					return ctx
				}
				pbooks := GetAllPlaybooksFromDB(s.playbookDB)
				for pname, pbook := range pbooks {
					if curpb.Name == pname && pbook.GetInstallState() {
						updates <- &executor.ExecutorUpdate{CurrentStep: pb.STEP_ERROR, StepMessage: "There is already a playbook named " + curpb.Name + "! Undo it first!"}
						return ctx
					}
				}
				if !curpb.Lock("Apply") {
					updates <- &executor.ExecutorUpdate{CurrentStep: pb.STEP_ERROR, StepMessage: "Unexpected lock on fresh playbook! (reason: " + curpb.GetLockReason() + ")"}
					return ctx
				}
				err = UpdatePlaybookDB(s.playbookDB, curpb)
				if err != nil {
					updates <- &executor.ExecutorUpdate{CurrentStep: pb.STEP_ERROR, StepMessage: "Failed adding playbook to db: " + err.Error() + ")"}
					return ctx
				}
				ctx = context.WithValue(ctx, "playbook", curpb)
				return ctx
			}))
			ex.AddStep(executor.NewStep(pb.STEP_FETCHIP, s.StepFetchIPs))
			ex.AddStep(executor.NewStep(pb.STEP_DNS, s.StepApplyDNS))
			ex.AddStep(executor.NewStep(pb.STEP_DNS, s.StepUpdatePlaybook))
			ex.AddStep(executor.NewStep(pb.STEP_ROUTES, s.StepApplyRoutes))
			ex.AddStep(executor.NewStep(pb.STEP_ROUTES, s.StepFinalizePlaybook)) // "finalize" here - set status as installed
		}
	default:
		s.reportStatus(ss, pb.STEP_ERROR, "Failed to build executor: task doesn't exist")
		return nil
	}
	if ex != nil { // Run & Report
		c := make(chan *executor.ExecutorUpdate)
		ex.Start(c)
		var err error
		for err == nil && ss.Context().Err() == nil {
			eupdctx, cancel := context.WithCancel(context.Background())
			go func() {
				for eupdctx.Err() == nil {
					upd := <-c
					s.reportStatus(ss, upd.CurrentStep, upd.StepMessage)
				}
			}()
			err = ex.Tick()
			if err != nil {
				if err.Error() == executor.ERR_FINISHED || err.Error() == executor.ERR_NOTSTART {
					cancel()
					break
				} else {
					log.Fatalln(err)
				}
			}
			err = ex.GetLastError()
			if err != nil {
				if err.Error() == executor.ERR_FINISHED || err.Error() == executor.ERR_NOTSTART {
					cancel()
					break
				} else {
					// Otherwise this error is from a step, let the pump process that
					err = nil
					time.Sleep(time.Millisecond * 10)
				}
			}
			cancel()
			time.Sleep(time.Millisecond * 10)
		}
		if ex.IsRunning() {
			return ss.Context().Err()
		}
	} else {
		s.reportStatus(ss, pb.STEP_ERROR, "Failed to run executor: executor is nil")
		return nil
	}
	return nil
}

/*
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
*/
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
