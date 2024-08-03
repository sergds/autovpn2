package client

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/sergds/autovpn2/internal/fastansi"
	"github.com/sergds/autovpn2/internal/playbook"
	pb "github.com/sergds/autovpn2/internal/rpc"
)

func Execute(task string, argv []string) {
	var summary []string = make([]string, 0)

	args := argv
	sp := fastansi.NewStatusPrinter()
	conn := ConnectToServer(sp)
	sp.Status(3, "Connected to AutoVPN @ "+conn.Target())
	sp.Status(1, "\t\t\t\t")
	sp.Status(0, "\t\t\t\t")
	sp.Status(2, "\t\t\t\t")
	//sp.PushLines()
	sp.Status(1, color.YellowString("Waiting for status..."))
	defer conn.Close()
	c := pb.NewAutoVPNClient(conn)

	switch task {
	case pb.TASK_APPLY:
		{
			pbc, err := os.ReadFile(args[0])
			if err != nil {
				fmt.Println(err.Error())
				os.Exit(0)
			}
			args[0] = string(pbc)
			sp.Status(2, color.YellowString("Applying playbook..."))

		}
	case pb.TASK_UNDO:
		pbname := args[0]
		// check if this is a filename
		if f, err := os.Open(pbname); err != nil {
			b, err := io.ReadAll(f)
			if err == nil {
				pb, err := playbook.Parse(string(b))
				if err != nil {
					pbname = pb.Name
				}
			}
		}
		// post process (a possible file name)
		pbname = strings.Split(pbname, string(os.PathSeparator))[len(strings.Split(pbname, string(os.PathSeparator)))-1]
		pbname = strings.Split(pbname, ".")[0]
		args[0] = pbname
		sp.Status(2, color.YellowString("Undoing playbook..."))
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ss, err := c.ExecuteTask(ctx, &pb.ExecuteRequest{Operation: task, Argv: args})
	if err != nil {
		log.Fatalln(err.Error())
	}
	for {
		status, err := ss.Recv()
		if err == io.EOF {
			fmt.Println("Done!")
			break
		}
		if err != nil {
			log.Fatalln(err.Error())
		}
		if status == nil {
			break
		}
		switch status.Statecode {
		case pb.STEP_ERROR:
			sp.Status(1, color.RedString("Error while applying playbook!"))
		case pb.STEP_NOTIFY:
			{
				sp.Status(1, color.BlueString(*status.Opdesc))
				continue
			}
		case pb.STEP_PUSH_SUMMARY:
			summary = append(summary, *status.Opdesc)
		default:
			sp.Status(1, *status.Statetext)
		}
		if status.Opdesc != nil {
			sp.Status(0, *status.Opdesc)
		}
	}
	if len(summary) != 0 {
		fmt.Println()
		fmt.Println("Operation Summary:")
		for _, s := range summary {
			fmt.Println(s)
		}
	}
}
