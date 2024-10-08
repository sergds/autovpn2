package dns

type DNSAdapter interface {
	Authenticate(conf map[string]string) error      // Some DNS setups may require credentials.
	GetRecords(dnstype string) ([]DNSRecord, error) // Get all records of type
	AddRecord(record DNSRecord) error               // Add a record to DNS
	DelRecord(record DNSRecord) error               // Delete a record from DNS
	CommitRecords() error                           // Like with routers, some DNS setups might not apply changes immediately.
}
