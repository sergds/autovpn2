# AutoVPN 2
![logo](logo.png)
A tool to easily manage VPN routed hosts in a small/home network with a local DNS (like Pi-Hole).

### Why?
It was created specifically for my needs, because of Russian Government blocking "illegal" websites, as well as western companies geoblocking russians (because of US Export Restrictions or other regulations.). 

This tool was made in order to evade these pointless blocks network-wide without wasting time manually verifying (and later updating) addesses.

Actually this is a second iteration of the autovpn toolset. The first autovpn had a one big list of domains to be "unblocked" and was a simple python script, which needed to be run periodally over ssh to keep Keenetic routes and Pi-hole DNS fresh. This became clumsy really fast, and after the rumors about russian government blocking YouTube reached me i decided to rewrite this in Go as a client-server app, with playbook approach to be able to scale it up a bit easier.

### Currently Available Adapters
DNS:
- PiholeAPI (Implementation of DNS Adapter for Pi-hole web API.)

Routes:
- KeeneticRCI (Implementation of routes adapter for Keenetic Remote Configuration Interface)

### Usage
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
# EXAMPLE OF A REAL AUTOVPN PLAYBOOK.
# Playbook to unblock youtube in Russia
name: YouTube
adapters: # Names for factory method
  routes: "KeeneticRCI"
  dns: "PiholeAPI"
adapterconfig: # Correspond to Authenticate() interface args
  routes:
    creds: "admin:password"
    endpoint: "http://10.0.2.1"
  dns:
    creds: "apikey"
    endpoint: "http://10.0.2.2:8080"
interface: Wireguard1
hosts: # List of hosts to route through VPN.
# Frontend
- www.youtube.com
- youtube.com
# User Images
- i.ytimg.com
- yt3.ggpht.com
- yt4.ggpht.com
- i9.ytimg.com
# Video CDN
- rr2---sn-8ph2xajvh-ut5l.googlevideo.com
```

### Implemented Features (more like TODO)
- [X] Routes
- [X] DNS
- [X] Try retrieving data from adapters for undo instead of it storing locally. (To avoid duplicate stray routes or dns records of different addresses)
- [ ] Allow specifying raw IPs in playbook's hosts
- [X] Store server playbooks in persistient cache (File? (Key-value) DB?)
- [ ] Auto-refreshing of playbook routes and DNS
- [ ] Clean code