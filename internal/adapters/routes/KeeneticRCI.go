package routes

import (
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"strings"

	"github.com/antonholmquist/jason"
)

// Implementation of routes adapter for Keenetic RCI (Remote Configuration Interface), which is basically a JSON RPC with OpenWRT UCI+Cisco like commands in JSON objects form.
// SergDS (C) 2024
type KeeneticRCI struct {
	endpoint string
	hclient  *http.Client
}

func newKeeneticRCI() *KeeneticRCI {
	jar, _ := cookiejar.New(&cookiejar.Options{})
	return &KeeneticRCI{hclient: &http.Client{Jar: jar}}
}

func (k *KeeneticRCI) Authenticate(creds string, endpoint string) bool {
	realcreds := strings.Split(creds, ":")
	if len(realcreds) != 2 {
		return false
	}
	k.endpoint = endpoint
	resp, err := k.hclient.Get(endpoint + "/auth")
	if err != nil {
		return false
	}
	if resp.StatusCode == 401 {
		md5h := md5.Sum([]byte(realcreds[0] + ":" + resp.Header.Get("X-NDM-Realm") + ":" + realcreds[1]))
		sha256h := sha256.Sum256([]byte(resp.Header.Get("X-NDM-Challenge") + hex.EncodeToString(md5h[:])))
		resp, err := k.hclient.Post(endpoint+"/auth", "application/json", strings.NewReader("{\"login\": \""+realcreds[0]+"\", \"password\": \""+hex.EncodeToString(sha256h[:])+"\"}"))
		if resp.StatusCode == 200 && err == nil {
			return true // we are in
		}
	}
	if resp.StatusCode == 200 {
		return true
	}
	return false
}

func (k *KeeneticRCI) GetRoutes() []Route {
	resp, err := k.hclient.Post(k.endpoint+"/rci/", "application/json", strings.NewReader("{\"show\":{\"ip\":{\"route\":{}},\"ipv6\":{\"route\":{}}}}"))
	if err != nil {
		return []Route{}
	}
	if resp.StatusCode != 200 {
		return []Route{}
	}
	b, _ := io.ReadAll(resp.Body)
	respjson, err := jason.NewObjectFromBytes(b)
	if err != nil {
		return []Route{}
	}
	routearr, err := respjson.GetValue("show", "ip", "route")
	if err != nil {
		fmt.Println(err.Error())
	}
	routeobj, _ := routearr.Marshal()
	var v4routes []Route
	json.Unmarshal(routeobj, &v4routes)
	return v4routes
}

// Some preformatted json ahead. Because arbitrary json handling in Go is kinda PAIN.

func (k *KeeneticRCI) AddRoute(route Route, comment string) bool {
	return k.rciRequest("[{\"ip\": {\"route\": {\"comment\": \"" + comment + "\", \"interface\": \"" + route.Interface + "\", \"host\": \"" + route.Destination + "\"}}}]")

}
func (k *KeeneticRCI) DelRoute(route Route) bool {
	return k.rciRequest("[{\"ip\": {\"route\": {\"interface\": \"" + route.Interface + "\", \"host\": \"" + route.Destination + "\", \"no\": \"true\", \"name\": \"" + route.Interface + "\"}}}]")
}

func (k *KeeneticRCI) SaveConfig() bool {
	return k.rciRequest("[{\"system\": {\"configuration\": {\"save\": True}}}]")
}

func (k *KeeneticRCI) rciRequest(contents string) bool {
	resp, err := k.hclient.Post(k.endpoint+"/rci/", "application/json", strings.NewReader(contents))
	if err != nil {
		fmt.Println(err.Error())
		return false
	}
	if resp.StatusCode == 200 {
		return true
	}
	return false
}
