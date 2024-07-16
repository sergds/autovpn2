package client

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"

	pb "github.com/sergds/autovpn2/internal/rpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func runApply(cl pb.AutoVPNClient, playbookpath string) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stream, err := cl.Apply(ctx, &pb.ApplyRequest{Playbook: playbookpath})
	if err != nil {
		log.Fatalln(err.Error())
	}
	for {
		status, err := stream.Recv()
		if err == io.EOF {
			fmt.Println("Done!")
			break
		}
		if err != nil {
			log.Fatalln(err.Error())
		}
		fmt.Println("Status Update :: ", status.Status, status.Statustext)
	}
}

func Apply(playbookpath string) {
	addrs := ResolveFirstAddr()

	if addrs == nil {
		fmt.Println("No servers found! Terminating!")
		os.Exit(0)
	}
	var conn *grpc.ClientConn

	for _, addr := range addrs {
		fmt.Println("Trying connecting to AutoVPN @ " + addr.String())

		var err error
		conn, err = grpc.NewClient(addr.String()+":15328", grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			continue
		}

	}
	fmt.Println("Connected to AutoVPN @ " + conn.Target())

	defer conn.Close()
	c := pb.NewAutoVPNClient(conn)

	runApply(c, playbookpath)
}
