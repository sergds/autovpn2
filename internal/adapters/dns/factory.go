package dns

import "strings"

func NewDNSAdapter(name string) DNSAdapter {
	n := strings.ToLower(name)
	switch n {
	case "piholeapi":
		{
			return newPiholeAPI()
		}
	case "null":
		{
			return newNullDNS()
		}
	default:
		{
			return newNullDNS()
		}
	}
	return nil
}
