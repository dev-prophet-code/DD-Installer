# Router and firewall guide

For players to find and join your server, the game ports must reach the machine that runs Dune Docker. DD-Installer can open the **server's own firewall** (ufw/firewalld) for you; your **router** and any **cloud firewall** you must configure yourself, because DD-Installer has no access to them.

## The ports

| Port | Protocol | Purpose | Who may reach it |
|---|---|---|---|
| `7777-7810` | UDP | game traffic and map instances | everyone |
| `31982` | TCP | RabbitMQ game messaging | everyone |
| `31983` | TCP | RabbitMQ game HTTP endpoint | everyone |
| `32000-32015` | UDP | optional direct latency checks (a relay is used when closed) | everyone (optional) |
| `8088` (or the port you chose) | TCP | **web admin console** | **only trusted administrators** |

Never expose PostgreSQL, RabbitMQ administration or Docker. The exact ports of your install: *Manage existing server... > Show status* prints `dune ports`.

## 1. The server firewall

On the Scan page tick **Open the game ports in the server firewall**. DD-Installer then:

* adds allow rules **only if ufw or firewalld is already active** - it never enables, disables or resets a firewall;
* allows your **SSH port first**, so you cannot lock yourself out;
* optionally opens the web console only for the IP or network you enter ("Web console allowed from", e.g. `203.0.113.7` or `203.0.113.0/24`). Leave it empty only if you must; the log then warns that the console is open to everyone.

Manually (Ubuntu with ufw):

```bash
sudo ufw allow 22/tcp
sudo ufw allow 7777:7810/udp
sudo ufw allow 32000:32015/udp
sudo ufw allow 31982/tcp
sudo ufw allow 31983/tcp
sudo ufw allow from 203.0.113.7 to any port 8088 proto tcp   # console: your IP only
sudo ufw status
```

## 2. A cloud firewall (VPS)

Most hosting providers have a firewall *in front of* your server: Hetzner **Cloud Firewall**, DigitalOcean **Cloud Firewalls**, AWS **Security Groups**, Azure **Network Security Groups**, Google Cloud **VPC firewall rules**, Oracle **Security Lists**, OVH **Network Firewall**. Create inbound rules for the table above (UDP and TCP are separate rules) and attach the firewall to your server. If you open nothing here, the server firewall setting alone is not enough.

## 3. Your home router (server at home)

1. Give the server a fixed address: reserve its LAN address in the router's DHCP settings (e.g. `192.168.1.50`).
2. Find **Port forwarding** (also called *virtual server*, *NAT*, *port mapping*).
3. Add one rule per row of the table above, protocol UDP or TCP as listed, destination = the server's LAN address.
4. Do **not** forward the web console port to the internet unless you restrict it; reach it from inside your network or through a VPN.
5. If your provider uses CGNAT or gives you only an IPv6 address, port forwarding at home will not work: use a VPS instead.

## 4. Test

* From a device **outside** your network (mobile data): can players see and join the server?
* If friends outside can join but you cannot from inside your own home network, your router lacks "hairpin NAT": see the upstream guide on [NAT loopback](https://docs.dunedocker.app/getting-started/nat-loopback).
* On the server: `ss -ltnup | grep -E '7777|3198'` shows the listeners.

Official reference: <https://docs.dunedocker.app/getting-started/networking>.
