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
)

func runApply(cl pb.AutoVPNClient, playbookpath string) {
	var summary []string = make([]string, 0)
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
		if status == nil {
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
		case pb.STATUS_PUSH_SUMMARY:
			summary = append(summary, *status.Statustext)
		}
		if err != nil {
			log.Fatalln(err.Error())
		}
		if status.Statustext != nil {
			sp.Status(0, *status.Statustext)
		}
	}
	fmt.Println()
	fmt.Println("Operation Summary:")
	for _, s := range summary {
		fmt.Println(s)
	}
}

func Apply(playbookpath string) {
	pbc, err := os.ReadFile(playbookpath)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(0)
	}
	sp := fastansi.NewStatusPrinter()
	conn := ConnectToServer(sp)
	sp.Status(0, "Connected to AutoVPN @ "+conn.Target())
	sp.Status(1, "\t\t\t\t")
	sp.PushLines()
	defer conn.Close()
	c := pb.NewAutoVPNClient(conn)

	runApply(c, string(pbc))
}
