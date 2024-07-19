package client

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"time"

	"github.com/grandcat/zeroconf"
	"github.com/sergds/autovpn2/internal/fastansi"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func ResolveFirstAddr() []net.IP {
	return ResolveAddr("")
}

func ResolveAddr(host string) []net.IP {
	sp := fastansi.NewStatusPrinter()
	sp.Status(1, "Resolving AutoVPN host(s) via mDNS...")
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		log.Fatalln(err.Error())
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*1)

	autovpns := make([]*zeroconf.ServiceEntry, 0)
	entries := make(chan *zeroconf.ServiceEntry)
	go func(results <-chan *zeroconf.ServiceEntry) {
		for entry := range results {
			autovpns = append(autovpns, entry)
			sp.Status(0, "Found so far: "+fmt.Sprint(len(autovpns)))
		}
		//log.Println("No more entries.")
	}(entries)

	defer cancel()
	err = resolver.Browse(ctx, "_autovpn._tcp", "local.", entries)
	if err != nil {
		sp.Status(0, "Failed to browse:", err.Error())
	}

	<-ctx.Done()

	if len(autovpns) == 0 {
		sp.Status(0, "Failed to detect AutoVPN servers through mDNS!")
		return nil
	}

	if host != "" {
		for _, h := range autovpns {
			for _, t := range h.Text {
				if t == host {
					return h.AddrIPv4
				}
			}
		}
	}
	sp.PushLines()
	return autovpns[0].AddrIPv4
}

func ConnectToServer(sp *fastansi.StatusPrinter) *grpc.ClientConn {
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
	return conn
}
