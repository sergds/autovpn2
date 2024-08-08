package dns

// Null DNS Adapter (no-op).
// Usable as a skeleton for new dns adapters for your device/setup.

type NullDNS struct {
}

func newNullDNS() *NullDNS {
	return &NullDNS{}
}

func (n *NullDNS) Authenticate(conf map[string]string) error { return nil }           // Some DNS setups may require credentials.
func (n *NullDNS) GetRecords(dnstype string) []DNSRecord     { return []DNSRecord{} } // Get all records of type
func (n *NullDNS) AddRecord(record DNSRecord) error          { return nil }           // Add a record to DNS
func (n *NullDNS) DelRecord(record DNSRecord) error          { return nil }           // Delete a record from DNS
func (n *NullDNS) CommitRecords() error                      { return nil }           // Like with routers, some DNS setups might not apply changes immediately.
