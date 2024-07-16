package client

import (
	"context"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/grandcat/zeroconf"
)

func ResolveFirstAddr() []net.IP {
	return ResolveAddr("")
}

func ResolveAddr(host string) []net.IP {
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		log.Fatalln(err.Error())
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*1)

	autovpns := make([]*zeroconf.ServiceEntry, 0)
	entries := make(chan *zeroconf.ServiceEntry)
	go func(results <-chan *zeroconf.ServiceEntry) {
		for entry := range results {
			//log.Println(entry)
			autovpns = append(autovpns, entry)
		}
		//log.Println("No more entries.")
	}(entries)

	defer cancel()
	err = resolver.Browse(ctx, "_autovpn._tcp", "local.", entries)
	if err != nil {
		log.Fatalln("Failed to browse:", err.Error())
	}

	<-ctx.Done()

	if len(autovpns) == 0 {
		fmt.Println("Failed to detect AutoVPN servers through mDNS!")
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

	return autovpns[0].AddrIPv4
}
