package dns

import "net"

// An oversimplified representation of a DNS record in a magical world without CNAME, SOA and other nightmares... contains only [A]ddress type, which is just fine for our goals.
type DNSRecord struct {
	Domain string `json:"domain"`
	Type   string `json:"type"`
	Addr   net.IP `json:"addr"`
	TTL    int    `json:"ttl"`
}
