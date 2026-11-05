package install

import (
	"fmt"
	"strings"

	"github.com/dev-prophet-code/dd-installer/internal/model"
)

// GamePorts are the ports Dune Docker needs reachable (upstream docs,
// "Networking and Ports"). The installer only ever ADDS allow rules.
var GamePorts = struct {
	UDPGame   string
	UDPProbe  string
	TCPGame   []int
	DefaultWB int
}{UDPGame: "7777:7810", UDPProbe: "32000:32015", TCPGame: []int{31982, 31983}, DefaultWB: 8088}

// firewallScript adds allow rules to an ALREADY ACTIVE host firewall (ufw or
// firewalld). It never enables, disables or resets a firewall, and it always
// allows the SSH port first so nobody is locked out.
func firewallScript(cfg model.Config, consolePort int) string {
	sshPort := 22
	if cfg.Target == model.TargetSSH && cfg.SSH.Port > 0 {
		sshPort = cfg.SSH.Port
	}
	from := strings.TrimSpace(cfg.ConsoleAllowFrom)
	var b strings.Builder
	b.WriteString(withPrelude("set -u\nSUDO=; [ \"$(id -u)\" -eq 0 ] || SUDO=sudo\n"))
	fmt.Fprintf(&b, "SSHP=%d\nCP=%d\nFROM=%s\n", sshPort, consolePort, shq(from))
	fmt.Fprintf(&b, "UDPG=%s\nUDPP=%s\n", shq(GamePorts.UDPGame), shq(GamePorts.UDPProbe))
	b.WriteString(`if command -v ufw >/dev/null 2>&1 && $SUDO ufw status 2>/dev/null | grep -q '^Status: active'; then
  echo "ufw is active - adding allow rules (nothing is removed or reset)."
  $SUDO ufw allow "$SSHP/tcp" comment 'DD-Installer: keep SSH open' >/dev/null
  $SUDO ufw allow "$UDPG/udp" comment 'Dune game traffic' >/dev/null
  $SUDO ufw allow "$UDPP/udp" comment 'Dune latency probes (optional)' >/dev/null
  for p in 31982 31983; do $SUDO ufw allow "$p/tcp" comment 'Dune messaging' >/dev/null; done
  if [ -n "$FROM" ]; then
    $SUDO ufw allow from "$FROM" to any port "$CP" proto tcp comment 'Dune web console (restricted)' >/dev/null
    echo "Web console port $CP is open only for $FROM."
  else
    $SUDO ufw allow "$CP/tcp" comment 'Dune web console' >/dev/null
    echo "WARNING: web console port $CP is open to everyone. Restrict it to trusted addresses if you can."
  fi
  echo "ufw rules added."
elif command -v firewall-cmd >/dev/null 2>&1 && $SUDO firewall-cmd --state 2>/dev/null | grep -q running; then
  echo "firewalld is running - adding allow rules (nothing is removed)."
  $SUDO firewall-cmd --permanent --add-port="$SSHP/tcp" >/dev/null
  $SUDO firewall-cmd --permanent --add-port="$(echo "$UDPG" | tr ':' '-')/udp" >/dev/null
  $SUDO firewall-cmd --permanent --add-port="$(echo "$UDPP" | tr ':' '-')/udp" >/dev/null
  for p in 31982 31983; do $SUDO firewall-cmd --permanent --add-port="$p/tcp" >/dev/null; done
  if [ -n "$FROM" ]; then
    $SUDO firewall-cmd --permanent --add-rich-rule="rule family=ipv4 source address=$FROM port port=$CP protocol=tcp accept" >/dev/null
    echo "Web console port $CP is open only for $FROM."
  else
    $SUDO firewall-cmd --permanent --add-port="$CP/tcp" >/dev/null
    echo "WARNING: web console port $CP is open to everyone. Restrict it to trusted addresses if you can."
  fi
  $SUDO firewall-cmd --reload >/dev/null
  echo "firewalld rules added."
else
  echo "No active ufw or firewalld found - nothing was changed."
  echo "If your provider has a cloud firewall / security group, open the ports there (see docs/ROUTER-AND-FIREWALL.md)."
fi
`)
	return b.String()
}

// verifyScript prints the resulting layout so the user (and the log) shows
// what the server actually has configured.
func verifyScript(cfg model.Config) string {
	return withPrelude(fmt.Sprintf(`set -u
cd "%s" 2>/dev/null || exit 0
echo "---- Sietches ----"
dgrp runtime/scripts/dune sietches list 2>&1 | head -n 40 || true
echo "---- Deep Desert ----"
dgrp runtime/scripts/dune deepdesert layout status 2>&1 | head -n 40 || true
echo "---- Ports ----"
dgrp runtime/scripts/dune ports 2>&1 | head -n 40 || true
exit 0
`, dirExpr(cfg)))
}
