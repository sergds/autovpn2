package client

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/fatih/color"
	"github.com/sergds/autovpn2/internal/fastansi"
	pb "github.com/sergds/autovpn2/internal/rpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func runApply(cl pb.AutoVPNClient, playbookpath string) {
	sp := fastansi.NewStatusPrinter()
	sp.Status(2, color.YellowString("Applying playbook..."))
	sp.Status(1, color.YellowString("Waiting for status..."))
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
		switch status.Status {
		case pb.STATUS_FETCHIP:
			sp.Status(1, color.YellowString("Fetching ip adressess..."))
		case pb.STATUS_DNS:
			sp.Status(1, color.YellowString("Updating DNS entries..."))
		case pb.STATUS_ROUTES:
			sp.Status(1, color.YellowString("Updating Routes..."))
		case pb.STATUS_ERROR:
			sp.Status(1, color.RedString("Error while applying playbook!"))
		case pb.STATUS_NOTIFY:
			{
				sp.Status(1, color.BlueString(*status.Statustext))
				continue
			}
		}
		if err != nil {
			log.Fatalln(err.Error())
		}
		if status.Statustext != nil {
			sp.Status(0, *status.Statustext)
		}
	}
}

func Apply(playbookpath string) {
	pbc, err := os.ReadFile(playbookpath)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(0)
	}
	sp := fastansi.NewStatusPrinter()
	addrs := ResolveFirstAddr()
	sp.Status(1, "Connecting...")
	if addrs == nil {
		sp.Status(0, "No servers found! Terminating!")
		os.Exit(0)
	}
	var conn *grpc.ClientConn

	for _, addr := range addrs {
		sp.Status(0, "Trying connecting to AutoVPN @ "+addr.String())

		var err error
		conn, err = grpc.NewClient(addr.String()+":15328", grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			continue
		}

	}
	sp.Status(0, "Connected to AutoVPN @ "+conn.Target())
	sp.Status(1, "\t\t\t\t")
	sp.PushLines()
	defer conn.Close()
	c := pb.NewAutoVPNClient(conn)

	runApply(c, string(pbc))
}
