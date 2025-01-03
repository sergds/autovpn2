# AutoVPN 2
![logo](misc/logov1/logo.png)
A tool to easily manage VPN routed hosts in a small/home network with a local DNS (like Pi-Hole).

### Why?
It was created specifically for my needs, because of Russian Government blocking "illegal" websites, as well as western companies geoblocking russians (because of US Export Restrictions or other regulations.). 

And if government blocks can be relatively easily evaded with packet modifications, the geoblocks can not.

This tool was made in order to evade these pointless blocks network-wide without wasting time manually verifying (and later updating) addresses.

Actually this is a second iteration of the autovpn toolset. The first autovpn had a one big list of domains to be "unblocked" and was a simple python script, which needed to be run periodally over ssh to keep Keenetic routes and Pi-hole DNS fresh. This became clumsy really fast, and after the rumors about russian government blocking YouTube reached me i decided to rewrite this in Go as a client-server app, with playbook approach to be able to scale it up a bit easier.

### Implemented Features (more like TODO)
- [X] Routes
- [X] DNS
- [X] Try retrieving data from adapters for undo instead of it storing locally. (To avoid duplicate stray routes or dns records of different addresses)
- [X] Allow specifying raw IPs in playbook's hosts
- [X] Store server playbooks in persistient cache (File? (Key-value) DB?)
- [ ] Auto-refreshing of playbook routes and DNS
- [ ] Clean code

### Currently Available Adapters
DNS:
- PiholeAPI (Implementation of DNS Adapter for Pi-hole web API.)

Routes:
- KeeneticRCI (Implementation of routes adapter for Keenetic Remote Configuration Interface)

### Usage
Server is automatically discovered via MDNS.
```
NAME:
   autovpn - autovpnupdater rewritten in go

USAGE:
   autovpn [global options] command [command options]

VERSION:
   2.0.0-alpha

COMMANDS:
   apply, a, ap, app      Apply local playbook to an autovpn environment.
   list, l, ls, lis       List of applied playbooks on an autovpn server.
   undo, u, und           Undo and remove playbook from server.
   server, s, serve, srv  Run autovpn server from here.
   help, h                Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --help, -h     show help
   --version, -v  print the version
```
### Example Playbook YAML
```yaml
# Playbook to bypass Netflix geoblock in west-east eu regions.
# non-web NRDP/API endpoints are likely outdated and need to be tested and adjusted.
name: netflix
adapters:
  routes: "keeneticrci"
  dns: "piholeapi"
adapterconfig:
  routes:
    keenetic_login: "admin:password"
    keenetic_origin: "http://10.0.2.1"
  dns:
    pihole_apikey: "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
    pihole_server: "http://10.0.2.2:8080"
interface: Wireguard1
autoupdateinterval: 24 # In hours. Auto-Update this playbook's ips every 24 hours. 0 Disables auto update. 
hosts:
# Frontend
- help.netflix.com
- uiboot.netflix.com
- www.netflix.com
- netflix.com
- assets.nflxext.com
- cdn-0.nflximg.com
# NRDP AppBoot and other bootstrap stuff
- nrdp52-appboot.netflix.com
- nrdp51-appboot.netflix.com
- android-appboot.netflix.com
- appboot.eu-west-1.origin.prodaa.netflix.com
- appboot.netflix.com
# Front-end APIs
- api-global.netflix.com
- push.prod.netflix.com
- android.prod.ftl.netflix.com
- ios.ngp.prod.cloud.netflix.com
- ios.prod.http1.netflix.com
- www.dradis.netflix.com
- prod.http1.dradis.netflix.com
- ios.prod.dradis.netflix.com
- ichnaea.netflix.com
- mobile-ixanycast.ftl.netflix.com
- android.prod.cloud.netflix.com
- web.prod.cloud.netflix.com
# Netflix Ready Device Platform
- nrdp.nccp.netflix.com
- nrdp.prod.cloud.netflix.com
- nrdp-ipv6.prod.ftl.netflix.com
# Hisense smart TV endpoint
- hisense-de9fa1c9.prod.partner.netflix.net
# Open Connect Control Planes
- occ-0-769-299.1.nflxso.net
- occ-0-1167-1168.1.nflxso.net
- occ-0-38-1501.1.nflxso.net
- occ-0-768-769.1.nflxso.net
# observed Open Connect Appliances (Edge video CDNs). Likely not needed, because open connect hosts are not geoblocked (in my experience).
- ipv4-c002-rix001-sia12578-isp.1.oca.nflxvideo.net
- ipv4-c004-rix001-sia12578-isp.1.oca.nflxvideo.net
```
