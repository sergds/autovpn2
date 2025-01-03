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
	"github.com/sergds/autovpn2/internal/playbook"
	pb "github.com/sergds/autovpn2/internal/rpc"
	"github.com/sergds/autovpn2/internal/server/executor"
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
	var builder *TaskBuilder = NewTaskBuilder(s)
	switch in.Operation { // Build Executor
	case pb.TASK_LIST:
		{
			err := builder.List()
			if err != nil {
				s.reportStatus(ss, pb.STEP_ERROR, err.Error())
				return err
			}
			ex = builder.Build()
		}
	case pb.TASK_APPLY:
		{
			err := builder.Apply(in.Argv[0])
			if err != nil {
				s.reportStatus(ss, pb.STEP_ERROR, err.Error())
				return err
			}
			ex = builder.Build()
		}
	case pb.TASK_UNDO:
		{
			err := builder.Undo(in.Argv[0])
			if err != nil {
				s.reportStatus(ss, pb.STEP_ERROR, err.Error())
				return err
			}
			ex = builder.Build()
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
