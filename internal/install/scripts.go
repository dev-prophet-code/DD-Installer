package install

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"github.com/dev-prophet-code/dd-installer/internal/probe"
	"sort"
	"strings"

	"github.com/dev-prophet-code/dd-installer/internal/model"
)

const upstreamBootstrap = "https://raw.githubusercontent.com/Red-Blink/dune-awakening-selfhost-docker/main/bootstrap.sh"

// prelude is prepended to every script. dgrp runs a command with Docker
// access even when the docker group membership is not active in this login
// session yet (right after the upstream installer added the user).
const prelude = `set -u
dgrp() {
  if docker info >/dev/null 2>&1; then "$@"; return $?; fi
  if command -v sg >/dev/null 2>&1 && getent group docker >/dev/null 2>&1; then
    c=""; for a in "$@"; do c="$c '$(printf %s "$a" | sed "s/'/'\\\\''/g")'"; done
    sg docker -c "$c"; return $?
  fi
  sudo -n "$@"
}
`

// dirExpr returns a double-quote-safe shell expression for the install dir.
// Model validation already rejects quotes, $, backticks and friends.
func dirExpr(cfg model.Config) string {
	switch {
	case cfg.InstallDir == "":
		return "$HOME/dune-awakening-selfhost-docker"
	case strings.HasPrefix(cfg.InstallDir, "~/"):
		return "$HOME/" + strings.TrimPrefix(cfg.InstallDir, "~/")
	default:
		return cfg.InstallDir
	}
}

// expand turns assistant placeholders into real shell.
func expand(script string, user string) string {
	script = strings.ReplaceAll(script, "@docker-group ", "dgrp ")
	script = strings.ReplaceAll(script, "@sudo-temp-grant", sudoGrantScript(user))
	return script
}

// withPrelude adds the shared helpers.
func withPrelude(s string) string { return prelude + s }

func randID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// heredoc embeds text literally (no expansion) with a random delimiter.
func heredoc(dst, content string) string {
	d := "DD_EOF_" + randID()
	return fmt.Sprintf("cat > %s <<'%s'\n%s\n%s\n", dst, d, content, d)
}

func bootstrapScript(cfg model.Config, reuse bool) string {
	d := dirExpr(cfg)
	var b strings.Builder
	b.WriteString(withPrelude("set -e\n"))
	fmt.Fprintf(&b, "DIR=\"%s\"\n", d)
	b.WriteString(`if [ -f "$DIR/install.sh" ]; then
  echo "Existing Dune Docker found in $DIR - reusing it (nothing is overwritten)."
  exit 0
fi
`)
	if reuse {
		b.WriteString("echo \"Folder $DIR exists but is incomplete; remove it manually or choose another folder.\" >&2; exit 1\n")
		return b.String()
	}
	// bootstrap.sh itself refuses root, existing dirs and low disk space.
	fmt.Fprintf(&b, "curl -fsSL --proto '=https' --retry 3 '%s' | DUNE_BOOTSTRAP_SKIP_INSTALL=1 DUNE_INSTALL_DIR=\"$DIR\" sh\n", upstreamBootstrap)
	return b.String()
}

func installScript(cfg model.Config, port int) string {
	return withPrelude(fmt.Sprintf(`set -e
cd "%[1]s"
ADMIN_BIND_PORT=%[2]d ./install.sh
P=$(sed -n 's/^ADMIN_BIND_PORT=//p' .env 2>/dev/null | tail -n1)
echo "DD_PORT ${P:-%[2]d}"
echo "DD_UPSTREAM_VERSION $(tr -d '[:space:]' < VERSION 2>/dev/null)"
`, dirExpr(cfg), port))
}

// checkBootstrapScript exits 0 when the release files are already in place.
func checkBootstrapScript(cfg model.Config) string {
	return fmt.Sprintf("[ -f \"%s/install.sh\" ]\n", dirExpr(cfg))
}

// checkInstallScript exits 0 when the Web Console already answers; it also
// reports the port it found so later steps use the right one.
func checkInstallScript(cfg model.Config, port int) string {
	return fmt.Sprintf(`cd "%[1]s" 2>/dev/null || exit 1
P=$(sed -n 's/^ADMIN_BIND_PORT=//p' .env 2>/dev/null | tail -n1)
P=${P:-%[2]d}
command -v curl >/dev/null 2>&1 || exit 1
curl -fsS -m 5 "http://127.0.0.1:$P/api/health" >/dev/null 2>&1 || exit 1
echo "DD_PORT $P"
echo "DD_UPSTREAM_VERSION $(tr -d '[:space:]' < VERSION 2>/dev/null)"
`, dirExpr(cfg), port)
}

// tuningScript writes .env overrides (values come only from the sizing
// package's fixed vocabulary; still validated defensively here).
func tuningScript(cfg model.Config, env map[string]string) string {
	if len(env) == 0 {
		return ""
	}
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteString(withPrelude("set -e\n"))
	fmt.Fprintf(&b, "cd \"%s\"\ntouch .env\n", dirExpr(cfg))
	for _, k := range keys {
		v := env[k]
		if !safeEnvKey(k) || !safeEnvVal(v) {
			continue
		}
		fmt.Fprintf(&b, "awk -F= '$1 != \"%s\"' .env > .env.dd.tmp && cat .env.dd.tmp > .env && rm -f .env.dd.tmp\n", k)
		fmt.Fprintf(&b, "printf '%%s\\n' '%s=\"%s\"' >> .env\n", k, v)
	}
	b.WriteString("echo \"Applied \"" + fmt.Sprint(len(keys)) + "\" tuning values to .env\"\n")
	return b.String()
}

func safeEnvKey(k string) bool {
	if k == "" {
		return false
	}
	for _, r := range k {
		if !(r == '_' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9') {
			return false
		}
	}
	return true
}

func safeEnvVal(v string) bool {
	for _, r := range v {
		ok := r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune(" +:._-=", r)
		if !ok {
			return false
		}
	}
	return true
}

// consoleScript runs the embedded Python helper for one phase.
// params carries secrets (token) and is written to a 0600 temp file.
func consoleScript(cfg model.Config, phase string, paramsJSON string) string {
	var b strings.Builder
	b.WriteString(withPrelude("set -e\n"))
	fmt.Fprintf(&b, "cd \"%s\"\n", dirExpr(cfg))
	b.WriteString("T=$(mktemp -d); umask 077; trap 'rm -rf \"$T\"' EXIT\n")
	b.WriteString(heredoc("\"$T/api.py\"", consoleHelper))
	b.WriteString("( umask 077; ")
	b.WriteString(heredoc("\"$T/params.json\"", paramsJSON))
	b.WriteString(")\n")
	fmt.Fprintf(&b, "PY=$(command -v python3 || command -v python)\n\"$PY\" \"$T/api.py\" \"$T/params.json\" %s\n", phase)
	return b.String()
}

// readyScript waits until `dune ready` succeeds.
func readyScript(cfg model.Config, minutes int) string {
	return withPrelude(fmt.Sprintf(`cd "%s"
i=0; max=%d
until dgrp runtime/scripts/dune ready >/dev/null 2>&1; do
  i=$((i+1)); [ "$i" -gt "$max" ] && { echo "DD_ERR battlegroup-not-ready"; dgrp runtime/scripts/dune ready 2>&1 | tail -n 30; exit 40; }
  [ $((i %% 6)) -eq 0 ] && echo "Waiting for the battlegroup to become ready ($((i/6)) min)..."
  sleep 10
done
echo "Battlegroup is ready."
`, dirExpr(cfg), minutes*6))
}

func postScript(cfg model.Config, cmds []string) string {
	if len(cmds) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(withPrelude("set -e\n"))
	fmt.Fprintf(&b, "cd \"%s\"\n", dirExpr(cfg))
	for _, c := range cmds {
		fmt.Fprintf(&b, "echo '> %s'\ndgrp %s\n", c, c)
	}
	return b.String()
}

// sudoGrantScript gives the user temporary passwordless sudo using the
// password supplied on stdin by the orchestrator (variable DD_SUDO_PW is set
// by the wrapper). It validates the file with visudo before activating it.
func sudoGrantScript(user string) string {
	return fmt.Sprintf(`set -e
[ -n "${DD_SUDO_PW:-}" ] || { echo "No sudo password available" >&2; exit 1; }
TMP=$(mktemp)
echo '%[1]s ALL=(ALL) NOPASSWD:ALL' > "$TMP"
printf '%%s\n' "$DD_SUDO_PW" | sudo -S -p '' sh -c "visudo -cf '$TMP' >/dev/null && install -m 0440 -o root -g root '$TMP' /etc/sudoers.d/90-dd-installer"
rm -f "$TMP"
sudo -n true
# safety net: remove the grant after 3 hours even if DD-Installer crashes or is closed
if command -v systemd-run >/dev/null 2>&1 && [ -d /run/systemd/system ]; then
  sudo -n systemd-run --quiet --unit=dd-installer-sudo-revoke --on-active=3h /bin/rm -f /etc/sudoers.d/90-dd-installer >/dev/null 2>&1 || true
else
  sudo -n sh -c 'nohup sh -c "sleep 10800; rm -f /etc/sudoers.d/90-dd-installer" >/dev/null 2>&1 &' || true
fi
echo "Temporary passwordless sudo enabled for %[1]s (removed when the installation ends, at the latest after 3 hours)."
`, user)
}

const sudoRevokeScript = `sudo -n rm -f /etc/sudoers.d/90-dd-installer 2>/dev/null || true
sudo -n systemctl stop dd-installer-sudo-revoke.timer 2>/dev/null || true
`

// consoleHelper talks to the Dune Docker Console API on 127.0.0.1 of the
// target, so no extra port has to be reachable from the installer machine.
const consoleHelper = `import json, sys, time, os, urllib.request, urllib.error, http.cookiejar

P = json.load(open(sys.argv[1]))
phase = sys.argv[2]
base = "http://127.0.0.1:%d" % P["port"]
cj = http.cookiejar.CookieJar()
op = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(cj))
csrf = None

def out(*a):
    print(*a, flush=True)

def call(method, path, body=None, timeout=120):
    data = None if body is None else json.dumps(body).encode()
    h = {"content-type": "application/json"}
    if csrf:
        h["x-csrf-token"] = csrf
    req = urllib.request.Request(base + path, data=data, method=method, headers=h)
    try:
        with op.open(req, timeout=timeout) as r:
            raw = r.read()
            return r.status, (json.loads(raw) if raw else {})
    except urllib.error.HTTPError as e:
        raw = e.read()
        try:
            return e.code, json.loads(raw)
        except Exception:
            return e.code, {"error": raw.decode("utf-8", "replace")[:300]}

def wait_health(limit=240):
    t0 = time.time()
    while time.time() - t0 < limit:
        try:
            s, _ = call("GET", "/api/health", timeout=5)
            if s == 200:
                return
        except Exception:
            pass
        time.sleep(3)
    out("DD_ERR console-not-reachable: connection refused on", base)
    sys.exit(20)

def login():
    global csrf
    try:
        pw = open(os.path.expandvars(P["passwordFile"])).read().strip()
    except PermissionError:
        out("permission denied reading", P["passwordFile"], "- runtime files are not writable by the install user")
        sys.exit(21)
    except FileNotFoundError:
        out("DD_ERR admin password file missing:", P["passwordFile"])
        sys.exit(21)
    s, j = call("POST", "/api/auth/login", {"password": pw})
    if s == 429:
        out("Too many sign-in attempts (HTTP 429)")
        sys.exit(22)
    if s != 200:
        out("DD_ERR login-failed HTTP", s, j.get("error", ""))
        sys.exit(22)
    csrf = j.get("csrfToken")

def task_wait(task, limit=4 * 3600, label=""):
    tid = task["id"]
    seen = 0
    t0 = time.time()
    while time.time() - t0 < limit:
        s, j = call("GET", "/api/setup/tasks/" + tid, timeout=30)
        t = j.get("task", {})
        lines = t.get("logLines", [])
        for l in lines[seen:]:
            out(l)
        seen = len(lines)
        st = t.get("status")
        if st == "succeeded":
            return
        if st == "failed":
            out("Task failed:", t.get("errorMessage") or "", "(exit %s)" % t.get("exitCode"))
            sys.exit(30)
        time.sleep(5)
    out("Task timed out", label)
    sys.exit(31)

def post_task(path, body):
    s, j = call("POST", path, body)
    if s not in (200, 202) or "task" not in j:
        out("Request", path, "failed: HTTP", s, j.get("error", ""))
        sys.exit(32)
    return j["task"]

if phase == "check-setup":
    try:
        s, j = call("GET", "/api/setup/state", timeout=8)
        done = s == 200 and bool((j.get("files") or {}).get("complete"))
    except Exception:
        done = False
    out("setup complete" if done else "setup not complete")
    sys.exit(0 if done else 1)

wait_health()
login()

def attach_running(op):
    s, j = call("GET", "/api/setup/tasks", timeout=30)
    for t in j.get("tasks", []):
        if t.get("operation") == op and t.get("status") in ("queued", "running"):
            return t
    return None

if phase == "setup":
    s, j = call("POST", "/api/setup/write-config", {"SERVER_TITLE": P["title"], "SERVER_REGION": P["region"]})
    if s != 200:
        out("write-config failed:", s, j.get("error", "")); sys.exit(32)
    out("Server identity saved.")
    s, j = call("POST", "/api/setup/save-token", {"token": P["token"]})
    if s != 200:
        out("DD_ERR token rejected:", s, j.get("error", "")); sys.exit(33)
    out("Funcom token saved.")
    running = attach_running("init")
    if running:
        out("A deployment is already running on the server - continuing to follow it.")
        task_wait(running, label="init")
    else:
        out("Deploying (downloads the game files - this can take a long time)...")
        task_wait(post_task("/api/setup/init", {}), label="init")
    out("Deployment complete.")

elif phase == "start":
    out("Starting the battlegroup...")
    task_wait(post_task("/api/server/start", {}), limit=3600, label="start")

elif phase == "layout":
    sietches = P["sietches"]
    if len(sietches) > 1:
        task_wait(post_task("/api/sietches/update", {"action": "set-max", "map": "Survival_1", "count": len(sietches), "confirmation": "UPDATE SIETCHES"}), 1800)
        task_wait(post_task("/api/sietches/update", {"action": "set-active", "map": "Survival_1", "count": len(sietches), "confirmation": "UPDATE SIETCHES"}), 3600)
    s, j = call("GET", "/api/sietches/dimensions?map=Survival_1&ids=1")
    ids = [l.strip() for l in (j.get("stdout") or "").splitlines() if l.strip().isdigit()]
    out("Sietch partitions:", ids)
    if len(ids) < len(sietches):
        out("Only %d of %d Sietch partitions are active yet." % (len(ids), len(sietches)))
        sys.exit(34)
    for pid, sc in zip(ids, sietches):
        task_wait(post_task("/api/sietches/update", {"action": "set-display", "partitionId": pid, "displayName": sc["name"], "confirmation": "UPDATE SIETCHES"}), 1800)
        out("Sietch", pid, "named", sc["name"])
        if P.get("combat"):
            pvp = sc["mode"] == "PVP"
            s, j = call("POST", "/api/maps/user-settings/save", {
                "scope": "partition", "map": "Survival_1", "partitionId": pid,
                "values": {"partition_pvp_enabled": "True" if pvp else "False", "partition_pve_enabled": "False" if pvp else "True"},
                "deferRestart": True})
            out("  ruleset", sc["mode"], "->", "saved" if s in (200, 202) else "FAILED (HTTP %s %s)" % (s, j.get("error", "")))
    dd = P.get("deepDesert") or {}
    if dd.get("instances"):
        body = {"instances": dd["instances"], "confirmation": "UPDATE DEEP DESERT"}
        if dd.get("thirdRole"):
            body["thirdRole"] = dd["thirdRole"].lower()
        task_wait(post_task("/api/deepdesert/update", body), 3600)
        out("Deep Desert layout set to", dd["instances"], "instance(s).")
    out("Layout applied. Restart-sensitive changes were saved without forcing a restart.")
`

// ProbeScript runs the host scan against the folder the install will use
// (or its nearest existing parent) so disk numbers are meaningful.
func ProbeScript(cfg model.Config) string {
	return fmt.Sprintf("D=\"%s\"\nwhile [ ! -d \"$D\" ] && [ \"$D\" != / ] && [ \"$D\" != . ]; do D=$(dirname \"$D\"); done\nDUNE_PROBE_DIR=\"$D\"; export DUNE_PROBE_DIR\nDUNE_PROBE_WEB_PORT=%d; export DUNE_PROBE_WEB_PORT\n%s", dirExpr(cfg), cfg.WebPort, probeScript())
}

func probeScript() string { return probe.Script }
