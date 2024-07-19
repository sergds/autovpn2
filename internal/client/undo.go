package client

import (
	"context"
	"fmt"
	"io"
	"log"

	"github.com/fatih/color"
	"github.com/sergds/autovpn2/internal/fastansi"
	pb "github.com/sergds/autovpn2/internal/rpc"
)

func runUndo(cl pb.AutoVPNClient, name string) {
	sp := fastansi.NewStatusPrinter()
	sp.Status(2, color.YellowString("Undoing playbook..."))
	sp.Status(1, color.YellowString("Waiting for status..."))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stream, err := cl.Undo(ctx, &pb.UndoRequest{Playbookname: name})
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
		case pb.UNDO_STATUS_DNS:
			sp.Status(1, color.YellowString("Removing DNS entries..."))
		case pb.UNDO_STATUS_ROUTES:
			sp.Status(1, color.YellowString("Removing Routes..."))
		case pb.UNDO_STATUS_ERROR:
			sp.Status(1, color.RedString("Error while undoing playbook!"))
		case pb.UNDO_STATUS_NOTIFY:
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

func Undo(playbookname string) {
	sp := fastansi.NewStatusPrinter()
	conn := ConnectToServer(sp)
	sp.Status(0, "Connected to AutoVPN @ "+conn.Target())
	sp.Status(1, "\t\t\t\t")
	sp.PushLines()
	defer conn.Close()
	c := pb.NewAutoVPNClient(conn)

	runUndo(c, playbookname)
}
