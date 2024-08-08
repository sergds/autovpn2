package routes

import (
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"strings"

	"github.com/antonholmquist/jason"
)

// Implementation of routes adapter for Keenetic RCI (Remote Configuration Interface), which is basically a JSON RPC with OpenWRT UCI+Cisco like commands in JSON objects form.
// SergDS (C) 2024
// Adapter config:
// keenetic_login -- login:password.
// keenetic_origin -- keenetic address, or keendns host.

type KeeneticRCI struct {
	endpoint string
	hclient  *http.Client
}

func newKeeneticRCI() *KeeneticRCI {
	jar, _ := cookiejar.New(&cookiejar.Options{})
	return &KeeneticRCI{hclient: &http.Client{Jar: jar}}
}

func (k *KeeneticRCI) Authenticate(conf map[string]string) error {
	realcreds := strings.Split(conf["keenetic_login"], ":")
	if len(realcreds) != 2 {
		return errors.New("wrong creds format (expected \"user:password\")")
	}
	k.endpoint = conf["keenetic_origin"]
	resp, err := k.hclient.Get(k.endpoint + "/auth")
	if err != nil {
		return err
	}
	if resp.StatusCode == 401 {
		md5h := md5.Sum([]byte(realcreds[0] + ":" + resp.Header.Get("X-NDM-Realm") + ":" + realcreds[1]))
		sha256h := sha256.Sum256([]byte(resp.Header.Get("X-NDM-Challenge") + hex.EncodeToString(md5h[:])))
		resp, err := k.hclient.Post(k.endpoint+"/auth", "application/json", strings.NewReader("{\"login\": \""+realcreds[0]+"\", \"password\": \""+hex.EncodeToString(sha256h[:])+"\"}"))
		if resp.StatusCode == 200 && err == nil {
			return nil // we are in
		}
	}
	if resp.StatusCode == 200 {
		return nil
	}
	return errors.New("authentication failed")
}

func (k *KeeneticRCI) GetRoutes() ([]*Route, error) {
	resp, err := k.hclient.Post(k.endpoint+"/rci/", "application/json", strings.NewReader("{\"show\":{\"ip\":{\"route\":{}},\"ipv6\":{\"route\":{}}}}"))
	if err != nil {
		return []*Route{}, err
	}
	if resp.StatusCode != 200 {
		return []*Route{}, errors.New("non 200 status code")
	}
	b, _ := io.ReadAll(resp.Body)
	respjson, err := jason.NewObjectFromBytes(b)
	if err != nil {
		return []*Route{}, err
	}
	routearr, err := respjson.GetValue("show", "ip", "route")
	if err != nil {
		fmt.Println(err.Error())
		return []*Route{}, err
	}
	routeobj, _ := routearr.Marshal()
	var v4routes []*Route
	json.Unmarshal(routeobj, &v4routes)
	// Strip network prefix. For our goals, raw ip is enough
	for _, rr := range v4routes {
		if strings.Contains(rr.Destination, "/") { // Catch v4 prefix
			rr.Destination = strings.Split(rr.Destination, "/")[0]
		}
	}
	// Comments are optional and stored separately. Get 'em
	routes2, err := k.rciRequestGET("ip/route")
	if err != nil {
		fmt.Println("error getting comments: " + err.Error())
		return v4routes, nil
	}
	//fmt.Println(routes2)
	routes2_parsed, err := jason.NewValueFromBytes([]byte(routes2))
	if err != nil {
		fmt.Println("error parsing routes2: " + err.Error())
		return v4routes, nil
	}
	routes2_arr, err := routes2_parsed.Array()
	if err != nil {
		fmt.Println("error parsing routes2 array: " + err.Error())
		return v4routes, nil
	}
	for _, r := range routes2_arr {
		robj, err := r.Object()
		if err != nil {
			fmt.Println(err.Error())
			break
		}
		comment, err := robj.GetString("comment")
		if err != nil {
			fmt.Println("Failed getting comment")
		}
		host, err := robj.GetString("host")
		if err != nil {
			fmt.Println("Failed getting host")
			continue
		}
		for _, rr := range v4routes {
			if rr.Destination == host {
				rr.Comment = comment
			}
		}
	}
	return v4routes, nil
}

// Some preformatted json ahead. Because arbitrary json handling in Go is kinda PAIN.

func (k *KeeneticRCI) AddRoute(route Route) error {
	return k.rciRequestJSON("[{\"ip\": {\"route\": {\"comment\": \"" + route.Comment + "\", \"interface\": \"" + route.Interface + "\", \"host\": \"" + route.Destination + "\"}}}]")

}
func (k *KeeneticRCI) DelRoute(route Route) error {
	return k.rciRequestJSON("[{\"ip\": {\"route\": {\"interface\": \"" + route.Interface + "\", \"host\": \"" + route.Destination + "\", \"no\": \"true\", \"name\": \"" + route.Interface + "\"}}}]")
}

func (k *KeeneticRCI) SaveConfig() error {
	return k.rciRequestJSON("[{\"system\": {\"configuration\": {\"save\": True}}}]")
}

func (k *KeeneticRCI) rciRequestJSON(contents string) error {
	resp, err := k.hclient.Post(k.endpoint+"/rci/", "application/json", strings.NewReader(contents))
	if err != nil {
		fmt.Println(err.Error())
		return err
	}
	if resp.StatusCode == 200 {
		return nil
	}
	return errors.New("non 200 status code")
}

// RCI allows GET requests with url path acting as a show command. These contain additional info for web ui.
// Route comments can only be retrieved this way.
func (k *KeeneticRCI) rciRequestGET(path string) (string, error) {
	resp, err := k.hclient.Get(k.endpoint + "/rci/" + path)
	if err != nil {
		fmt.Println(err.Error())
		return "", err
	}
	if resp.StatusCode != 200 {
		return "", errors.New("non 200 status code")
	}
	respstr, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println(err.Error())
		return "", err
	}
	return string(respstr), nil
}
