package dns

import "net"

type DNSRecord struct {
	Domain string `json:"domain"`
	Type   string `json:"type"`
	Addr   net.IP `json:"addr"`
	TTL    int    `json:"ttl"`
}
