# AutoVPN 2
A tool to manage VPN routed hosts.

It was created specifically for my needs, because of Russian Government blocking "illegal" websites, as well as western companies geoblocking russians (because of US Export Restrictions OR believing that a geoblock will stop the war.). 

So in order to evade these pointless blocks network-wide i created this tool.

 Actually this is a second iteration of autovpn toolset. The first autovpn had a one big list of domains to be "unblocked" and was a simple python script, which needed to be run periodally over ssh to keep Keenetic routes and Pi-hole DNS fresh. This became clumsy really fast, so i rewrote this in Go as a server-client app, with playbook approach.

### Usage
```
NAME:
   autovpn - autovpnupdater rewritten in go

USAGE:
   autovpn [global options] command [command options]

VERSION:
   2.0.0-dev

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

### (Implemented) Features (more like TODO)
- [X] Routes
- [X] DNS
- [ ] Try retrieving data from adapters for undo instead of it storing locally. (To avoid stray routes or dns records of different addresses)
- [ ] Auto-refreshing of routes
- [ ] Clean code