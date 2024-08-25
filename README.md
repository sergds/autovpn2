# AutoVPN 2
![logo](logo.png)
A tool to easily manage VPN routed hosts in a small/home network with a local DNS (like Pi-Hole).

### Why?
It was created specifically for my needs, because of Russian Government blocking "illegal" websites, as well as western companies geoblocking russians (because of US Export Restrictions or other regulations.). 

And if government blocks can be relatively easily evaded with packet modifications, the geoblocks can not.

This tool was made in order to evade these pointless blocks network-wide without wasting time manually verifying (and later updating) addresses.

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
# TODO
```

### Implemented Features (more like TODO)
- [X] Routes
- [X] DNS
- [X] Try retrieving data from adapters for undo instead of it storing locally. (To avoid duplicate stray routes or dns records of different addresses)
- [X] Allow specifying raw IPs in playbook's hosts
- [X] Store server playbooks in persistient cache (File? (Key-value) DB?)
- [ ] Auto-refreshing of playbook routes and DNS
- [ ] Clean code