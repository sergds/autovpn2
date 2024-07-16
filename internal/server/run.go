package server

import (
	"context"
	"log"
	"net"
	"os"

	"github.com/grandcat/zeroconf"
	pb "github.com/sergds/autovpn2/internal/rpc"
	"google.golang.org/grpc"
)

type AutoVPNServer struct {
	pb.UnimplementedAutoVPNServer
}

func (*AutoVPNServer) Apply(in *pb.ApplyRequest, ss pb.AutoVPN_ApplyServer) error {
	return nil
}

func (*AutoVPNServer) List(ctx context.Context, in *pb.ListRequest) (*pb.ListResponse, error) {
	return &pb.ListResponse{Playbooks: []string{"todo"}}, nil
}

func (*AutoVPNServer) Undo(in *pb.UndoRequest, ss pb.AutoVPN_UndoServer) error {
	return nil
}

func ServerMain() {
	lis, err := net.Listen("tcp", "0.0.0.0:15328")
	if err != nil {
		log.Fatalln(err.Error())
	}
	s := grpc.NewServer()
	pb.RegisterAutoVPNServer(s, &AutoVPNServer{})
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
