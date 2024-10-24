package main

import (
	"fmt"
	"log"
	"os"

	"github.com/sergds/autovpn2/internal"
	"github.com/sergds/autovpn2/internal/client"
	"github.com/sergds/autovpn2/internal/rpc"
	"github.com/sergds/autovpn2/internal/server"
	"github.com/urfave/cli/v2"
)

// Where it all begins...
func main() {
	fmt.Print("\n\n")
	app := &cli.App{
		Name:    "autovpn",
		Usage:   "autovpnupdater rewritten in go",
		Version: internal.Version(),
		Commands: []*cli.Command{
			{
				Name:    "apply",
				Aliases: []string{"a", "ap", "app"},
				Usage:   "Apply local playbook to an autovpn environment.",
				Action: func(ctx *cli.Context) error {
					if ctx.NArg() != 0 {
						client.Execute(rpc.TASK_APPLY, ctx.Args().Slice())
						os.Exit(0)
					} else {
						fmt.Println("Please specify path to a playbook!")
					}
					return nil
				},
			},
			{
				Name:    "list",
				Aliases: []string{"l", "ls", "lis"},
				Usage:   "List of applied playbooks on an autovpn server.",
				Action: func(ctx *cli.Context) error {
					client.Execute(rpc.TASK_LIST, ctx.Args().Slice())
					os.Exit(0)
					return nil
				},
			},
			{
				Name:    "undo",
				Aliases: []string{"u", "und"},
				Usage:   "Undo and remove playbook from server.",
				Action: func(ctx *cli.Context) error {
					if ctx.NArg() == 0 {
						fmt.Println("Missing playbook name!")
						os.Exit(0)
					}
					client.Execute(rpc.TASK_UNDO, ctx.Args().Slice())
					os.Exit(0)
					return nil
				},
			},
			{
				Name:    "server",
				Aliases: []string{"s", "serve", "srv"},
				Usage:   "Run autovpn server from here.",
				Action: func(ctx *cli.Context) error {
					server.ServerMain()
					os.Exit(0)
					return nil
				},
			},
		},
	}
	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
