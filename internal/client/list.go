package client

import (
	"context"
	"fmt"
	"os"

	"github.com/sergds/autovpn2/internal/fastansi"
	pb "github.com/sergds/autovpn2/internal/rpc"
)

func List() {
	sp := fastansi.NewStatusPrinter()
	conn := ConnectToServer(sp)
	sp.Status(0, "Connected to AutoVPN @ "+conn.Target())
	sp.Status(1, "\t\t\t\t")
	sp.PushLines()
	defer conn.Close()
	c := pb.NewAutoVPNClient(conn)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	lr, err := c.List(ctx, &pb.ListRequest{})
	if err != nil {
		fmt.Println("Failed to get playbook list: " + err.Error())
		os.Exit(0)
	}
	fmt.Println("Remote playbooks: ")
	for _, pbook := range lr.Playbooks {
		fmt.Println(pbook)
	}
}
