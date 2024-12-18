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
	updater    *AutoUpdater
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
	var pb_updated bool = false
	switch in.Operation { // Build Executor
	case pb.TASK_LIST:
		ex.AddStep(executor.NewStep(pb.STEP_LIST, s.StepList))
	case pb.TASK_APPLY:
		{
			// yep, that's definitely a HACK!
			curpb, err := playbook.Parse(in.Argv[0])
			if err != nil {
				s.reportStatus(ss, pb.STEP_ERROR, err.Error())
				return err
			}
			pbooks := GetAllPlaybooksFromDB(s.playbookDB)
			for pname, pbook := range pbooks {
				if curpb.Name == pname && pbook.GetInstallState() {
					pb_updated = true
				}
			}
			ex.AddStep(executor.NewStep("prep_ctx", func(updates chan *executor.ExecutorUpdate, ctx context.Context) context.Context { // TODO: Should I introduce new step const for these?
				curpb, err := playbook.Parse(in.Argv[0])
				if err != nil {
					updates <- &executor.ExecutorUpdate{CurrentStep: pb.STEP_ERROR, StepMessage: "Failed to parse playbook!"}
					return ctx
				}
				pbooks := GetAllPlaybooksFromDB(s.playbookDB)
				for pname, pbook := range pbooks {
					if curpb.Name == pname && pbook.GetInstallState() {
						updates <- &executor.ExecutorUpdate{CurrentStep: pb.STEP_PUSH_SUMMARY, StepMessage: "There is already a playbook named " + curpb.Name + "! Updating it!"}
						ctx = context.WithValue(ctx, "old_playbook", pbook)
					}
				}
				if !curpb.Lock("Apply") {
					updates <- &executor.ExecutorUpdate{CurrentStep: pb.STEP_ERROR, StepMessage: "Unexpected lock on fresh playbook! (reason: " + curpb.GetLockReason() + ")"}
					return ctx
				}
				err = UpdatePlaybookDB(s.playbookDB, curpb)
				if err != nil {
					updates <- &executor.ExecutorUpdate{CurrentStep: pb.STEP_ERROR, StepMessage: "Failed adding playbook to db: " + err.Error()}
					return ctx
				}
				ctx = context.WithValue(ctx, "playbook", curpb)
				return ctx
			}))
			// Just to be sure. Apply steps ~should~ handle old addrs, but potentially can lead to stray routes or dns records in the long run when undoing time comes.
			if pb_updated {
				ex.AddStep(executor.NewStep("swap", s.StepSwapPlaybooks))
				ex.AddStep(executor.NewStep(pb.UNDO_STEP_DNS, s.StepUndoDNS))
				ex.AddStep(executor.NewStep(pb.UNDO_STEP_ROUTES, s.StepUndoRoutes))
				ex.AddStep(executor.NewStep("swap", s.StepSwapPlaybooks)) // At this point old_playbook is no longer needed and was overwritten, so no need to unlock and stuff.
				// Continue with new one as usual.
			}
			ex.AddStep(executor.NewStep(pb.STEP_FETCHIP, s.StepFetchIPs))
			ex.AddStep(executor.NewStep(pb.STEP_DNS, s.StepApplyDNS))
			ex.AddStep(executor.NewStep(pb.STEP_DNS, s.StepUpdatePlaybook))
			ex.AddStep(executor.NewStep(pb.STEP_ROUTES, s.StepApplyRoutes))
			ex.AddStep(executor.NewStep(pb.STEP_ROUTES, s.StepFinalizePlaybook)) // "finalize" here - set status as installed and unlock
		}
	case pb.TASK_UNDO:
		{
			ex.AddStep(executor.NewStep("prep_ctx", func(updates chan *executor.ExecutorUpdate, ctx context.Context) context.Context { // TODO: Should I introduce new step const for these?
				var ok bool = false
				var wasinstalled bool = false
				var curpb *playbook.Playbook = nil
				pbooks := GetAllPlaybooksFromDB(s.playbookDB)

				for _, pbook := range pbooks {
					if pbook.Name == in.Argv[0] {
						ok = true
						curpb = pbook
						if pbook.GetInstallState() {
							wasinstalled = true
						}
					}
				}
				if !ok {
					updates <- &executor.ExecutorUpdate{CurrentStep: pb.STEP_ERROR, StepMessage: "No such playbook " + in.Argv[0] + " installed!"}
					return ctx
				}
				if !wasinstalled {
					updates <- &executor.ExecutorUpdate{CurrentStep: pb.STEP_ERROR, StepMessage: "Such playbook exists, but didn't finish installing! Removing!"}
					DeletePlaybookDB(s.playbookDB, curpb)
					return ctx
				}
				if !curpb.Lock("Undo") {
					updates <- &executor.ExecutorUpdate{CurrentStep: pb.STEP_ERROR, StepMessage: "Playbook is being processed at the moment (reason: " + curpb.GetLockReason() + ")!"}
					return ctx
				}
				err := UpdatePlaybookDB(s.playbookDB, curpb)
				if err != nil {
					updates <- &executor.ExecutorUpdate{CurrentStep: pb.STEP_ERROR, StepMessage: "Failed adding playbook to db: " + err.Error()}
					return ctx
				}
				s.UpdateUpdaterTable()
				ctx = context.WithValue(ctx, "playbook", curpb)
				return ctx
			}))
			ex.AddStep(executor.NewStep(pb.UNDO_STEP_DNS, s.StepUpdatePlaybook))
			ex.AddStep(executor.NewStep(pb.UNDO_STEP_DNS, s.StepUndoDNS))
			ex.AddStep(executor.NewStep(pb.UNDO_STEP_ROUTES, s.StepUndoRoutes))
			ex.AddStep(executor.NewStep("finalize", func(updates chan *executor.ExecutorUpdate, ctx context.Context) context.Context {
				curpb := ctx.Value("playbook").(*playbook.Playbook)
				err := DeletePlaybookDB(s.playbookDB, curpb)
				if err != nil {
					updates <- &executor.ExecutorUpdate{CurrentStep: pb.STEP_ERROR, StepMessage: "Failed removing playbook from db: " + err.Error()}
				}
				return ctx
			}))
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
	s.UpdateUpdaterTable()
	return nil
}

func (s *AutoVPNServer) UpdateUpdaterTable() {
	log.Println("Updating autoupdater ")
	books := GetAllPlaybooksFromDB(s.playbookDB)
	for name, pbook := range books {
		if pbook.GetInstallState() && pbook.GetLockReason() == "" {
			log.Println("Adding updater entry: " + name + " :: " + fmt.Sprint(pbook.Autoupdateinterval) + " hour(s)")
			s.updater.UpdateEntry(name, pbook.Autoupdateinterval)
		}
	}
	// Clean up removed.
	for name, _ := range s.updater.GetEntries() {
		ispresent := false
		for name2_new, pb := range books {
			if name2_new == name && pb.GetInstallState() && pb.GetLockReason() == "" {
				ispresent = true
			}
		}
		if !ispresent {
			log.Println("Collecting stale garbage entry from updater table: " + name)
			s.updater.DelEntry(name)
		}
	}
}

func (s *AutoVPNServer) UpdaterLoop() {
	for {
		time.Sleep(1 * time.Second)
		s.updater.Tick()
	}
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
	srv := &AutoVPNServer{playbookDB: pbdb}
	upd := NewAutoUpdater(srv)
	srv.updater = upd
	go srv.UpdaterLoop()
	pb.RegisterAutoVPNServer(s, srv)
	host, _ := os.Hostname()
	server, err := zeroconf.Register("AutoVPN Server @ "+host, "_autovpn._tcp", "local.", 15328, []string{"txtv=0", "host=" + host}, nil)
	defer server.Shutdown()
	if err != nil {
		log.Fatalln("Failed to initialize mDNS:", err.Error())
	}
	srv.UpdateUpdaterTable()
	log.Printf("autovpn server running @ %s", lis.Addr().String())
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
