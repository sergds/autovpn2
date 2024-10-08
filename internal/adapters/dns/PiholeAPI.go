package dns

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"

	"github.com/antonholmquist/jason"
)

// Implementation of DNS Adapter for Pi-hole web API.
// SergDS (C) 2024
// Adapter config:
// pihole_apikey -- Api key for pihole instance
// pihole_server -- Instance IP address
type PiholeAPI struct {
	apikey   string
	endpoint string
	hclient  *http.Client
}

func newPiholeAPI() *PiholeAPI {
	jar, _ := cookiejar.New(&cookiejar.Options{})
	return &PiholeAPI{hclient: &http.Client{Jar: jar}}
}

func (p *PiholeAPI) piholeRequest(args []string) (string, error) {
	requrl := p.endpoint + "/admin/api.php" + "?auth=" + p.apikey
	for _, arg := range args {
		requrl += "&" + arg
	}
	//fmt.Println(requrl)
	_, err := url.Parse(requrl)
	if err != nil {
		fmt.Println(err.Error())
		return "", err
	}
	resp, err := p.hclient.Get(requrl)
	if err != nil {
		fmt.Println(err.Error())
		return "", err
	}
	reqb, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println(err.Error())
		return "", err
	}
	if resp.StatusCode != 200 { // Their crappy api almost always returns 200. Even if not authorized, but who knows, maybe this if is not that useless.
		fmt.Println("pihole api: statuscode != 200")
		return "", err
	}
	return string(reqb), nil
}

func (p *PiholeAPI) Authenticate(conf map[string]string) error {
	p.apikey = conf["pihole_apikey"]
	p.endpoint = conf["pihole_server"]
	resp, ok := p.piholeRequest([]string{"customdns"}) // Use customdns command from base api to check auth token
	if ok != nil || resp == "Not authorized!" {        // Checking response string, because status code will be 200, even if it fails.
		fmt.Println(resp)
		return errors.New("Unauthorized")
	}
	return nil
}

func (p *PiholeAPI) GetRecords(dnstype string) ([]DNSRecord, error) {
	resp, ok := p.piholeRequest([]string{"customdns", "action=get"})
	if ok != nil {
		fmt.Println(ok.Error())
		return nil, ok
	}
	recs, err := jason.NewObjectFromBytes([]byte(resp))
	if err != nil {
		fmt.Println(err.Error())
		return nil, err
	}
	recsarrval, err := recs.GetValueArray("data")
	if err != nil {
		fmt.Println(err.Error())
		return nil, err
	}
	var finalrecords []DNSRecord
	for _, rec := range recsarrval {
		arr, _ := rec.Array()
		d, _ := arr[0].String()
		a, _ := arr[1].String()
		finalrecords = append(finalrecords, DNSRecord{Domain: d, Type: "A", Addr: net.ParseIP(a)})
	}
	return finalrecords, nil
}

func (p *PiholeAPI) AddRecord(record DNSRecord) error {
	resp, ok := p.piholeRequest([]string{"customdns", "action=add", "ip=" + record.Addr.String(), "domain=" + record.Domain})
	respparsed, err := jason.NewObjectFromBytes([]byte(resp))
	if err != nil {
		return err
	}
	errormsg, err := respparsed.GetString("message")
	if err != nil {
		fmt.Println("failed parsing status response")
	}
	if ok == nil && errormsg == "" {
		return nil
	}
	return ok
}

func (p *PiholeAPI) DelRecord(record DNSRecord) error {
	resp, ok := p.piholeRequest([]string{"customdns", "action=delete", "ip=" + record.Addr.String(), "domain=" + record.Domain})
	respparsed, err := jason.NewObjectFromBytes([]byte(resp))
	if err != nil {
		return err
	}
	errormsg, err := respparsed.GetString("message")
	if err != nil {
		fmt.Println("failed parsing status response")
	}
	if ok == nil && errormsg == "" {
		return nil
	}
	return ok
}

func (p *PiholeAPI) CommitRecords() error {
	return nil // Pi-hole api applies things in-place
}
