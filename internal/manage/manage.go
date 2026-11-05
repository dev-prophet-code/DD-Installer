// Package manage holds actions for an existing Dune Docker server (update,
// repair, backup, stop/uninstall) and for Funcom's official Hyper-V server
// (Director travel fix). Every action is an explicit user click; destructive
// ones need a typed confirmation phrase.
package manage

import (
	"fmt"
	"strings"

	"github.com/dev-prophet-code/dd-installer/internal/install"
	"github.com/dev-prophet-code/dd-installer/internal/model"
)

// Risk of an action, shown to the user.
type Risk string

const (
	Safe        Risk = "safe"        // read-only or harmless
	Disruptive  Risk = "disruptive"  // restarts services / disconnects players
	Destructive Risk = "destructive" // removes things; needs a typed phrase
)

// Group tells which kind of server an action is for.
type Group string

const (
	DuneDocker Group = "Dune Docker server"
	HyperV     Group = "Funcom Hyper-V server (official)"
)

// Action is one management operation.
type Action struct {
	ID          string
	Group       Group
	Title       string
	Description string
	Risk        Risk
	// Phrase must be typed by the user before a Destructive action runs.
	Phrase string
	Script func(cfg model.Config) string
}

// Hyper-V Director travel fix: pinned to a reviewed commit of
// Red-Blink/dune-awakening-hyperv-director-travel-fix.
const (
	HyperVFixRepo   = "Red-Blink/dune-awakening-hyperv-director-travel-fix"
	HyperVFixCommit = "9b6415ff44147e25689c12d5e1e66b268192244b"
)

func inDir(cfg model.Config, body string) string {
	return install.WithPrelude("set -u\ncd \"" + install.DirExpr(cfg) + "\" 2>/dev/null || { echo \"Dune Docker was not found in " + install.DirExpr(cfg) + "\" >&2; exit 1; }\n" + body)
}

// duneCmd runs one dune CLI command with Docker access.
func duneCmd(args string) string {
	return "echo '> dune " + args + "'\ndgrp runtime/scripts/dune " + args + "\n"
}

// Actions lists everything available, in display order.
func Actions() []Action {
	return []Action{
		{ID: "status", Group: DuneDocker, Title: "Show status and health", Risk: Safe,
			Description: "Runs `dune status`, `dune ready` and `dune doctor` and prints the result.",
			Script: func(c model.Config) string {
				return inDir(c, duneCmd("status")+duneCmd("ready")+duneCmd("doctor"))
			}},
		{ID: "backup", Group: DuneDocker, Title: "Create a database backup", Risk: Safe,
			Description: "Runs `dune db backup`. Backups are stored on the server in the Dune Docker folder.",
			Script:      func(c model.Config) string { return inDir(c, duneCmd("db backup")) }},
		{ID: "repair", Group: DuneDocker, Title: "Repair file permissions", Risk: Safe,
			Description: "Runs upstream's repair-host-runtime-permissions.sh (fixes root-owned runtime files after running commands with sudo).",
			Script: func(c model.Config) string {
				return inDir(c, "echo '> repair-host-runtime-permissions.sh'\ndgrp runtime/scripts/repair-host-runtime-permissions.sh\n")
			}},
		{ID: "update-check", Group: DuneDocker, Title: "Check for game and console updates", Risk: Safe,
			Description: "Runs `dune update check` and `dune self-update check`.",
			Script: func(c model.Config) string {
				return inDir(c, duneCmd("update check")+duneCmd("self-update check"))
			}},
		{ID: "update-game", Group: DuneDocker, Title: "Update the game files", Risk: Disruptive,
			Description: "Runs `dune update --yes`. The game servers restart and connected players are disconnected. Create a backup first.",
			Script:      func(c model.Config) string { return inDir(c, duneCmd("update --yes")) }},
		{ID: "update-console", Group: DuneDocker, Title: "Update Dune Docker itself", Risk: Disruptive,
			Description: "Runs `dune self-update install latest`. Note: DD-Installer is tested with one upstream version; newer ones may behave differently.",
			Script:      func(c model.Config) string { return inDir(c, duneCmd("self-update install latest")) }},
		{ID: "restart", Group: DuneDocker, Title: "Restart the battlegroup", Risk: Disruptive,
			Description: "Runs `dune stop` and then `dune start`. Players are disconnected.",
			Script:      func(c model.Config) string { return inDir(c, duneCmd("stop")+duneCmd("start")) }},
		{ID: "stop", Group: DuneDocker, Title: "Stop everything (keep all data)", Risk: Disruptive,
			Description: "Stops the battlegroup and removes the Web Console container. Files, backups and databases stay; start again with `./install.sh`.",
			Script: func(c model.Config) string {
				return inDir(c, duneCmd("stop")+"echo '> docker compose down (console)'\ndgrp docker compose -f docker-compose.web.yml down\n")
			}},
		{ID: "uninstall-all", Group: DuneDocker, Title: "Remove everything including data", Risk: Destructive, Phrase: "DELETE ALL",
			Description: "IRREVERSIBLY stops all containers, deletes the Docker volumes (database!) and removes the whole Dune Docker folder. Create and download a backup first.",
			Script:      uninstallAllScript},

		{ID: "hyperv-fix", Group: HyperV, Title: "Install the Director travel fix", Risk: Disruptive,
			Description: "For Funcom's official Hyper-V server (run against the game-server VM over SSH): fixes repeated password prompts when travelling between Overmap and Hagga Basin. Restarts the battlegroup. Community fix by RedBlink.",
			Script:      func(model.Config) string { return hypervScript("install") }},
		{ID: "hyperv-status", Group: HyperV, Title: "Check the Director travel fix", Risk: Safe,
			Description: "Runs the fix's `status` command.", Script: func(model.Config) string { return hypervScript("status") }},
	}
}

// ByID finds an action.
func ByID(id string) (Action, bool) {
	for _, a := range Actions() {
		if a.ID == id {
			return a, true
		}
	}
	return Action{}, false
}

// uninstallAllScript has hard guard rails: it refuses to delete anything that
// does not look like a Dune Docker folder, never "/" or the home directory.
func uninstallAllScript(cfg model.Config) string {
	dir := install.DirExpr(cfg)
	return install.WithPrelude(fmt.Sprintf(`set -u
DIR="%s"
fail() { echo "Refusing to continue: $1" >&2; exit 1; }
case "$DIR" in ""|"/"|"$HOME"|"$HOME/"|"/home"|"/root"|"/usr"|"/etc"|"/var"|"/opt") fail "unsafe folder '$DIR'";; esac
[ -d "$DIR" ] || fail "$DIR does not exist"
[ -f "$DIR/install.sh" ] && [ -f "$DIR/docker-compose.yml" ] && [ -d "$DIR/runtime" ] || fail "$DIR does not look like a Dune Docker folder"
cd "$DIR"
echo '> stopping the battlegroup'
dgrp runtime/scripts/dune stop || true
echo '> removing containers and volumes'
dgrp docker compose -f docker-compose.web.yml down -v || true
dgrp docker compose down -v || true
cd "$HOME"
echo "> deleting $DIR"
rm -rf "$DIR" 2>/dev/null || sudo -n rm -rf "$DIR" || fail "could not delete $DIR (permissions?)"
echo "Dune Docker was removed."
`, dir))
}

func hypervScript(cmd string) string {
	url := "https://github.com/" + HyperVFixRepo + "/archive/" + HyperVFixCommit + ".tar.gz"
	var b strings.Builder
	b.WriteString("set -u\n")
	if cmd == "install" {
		b.WriteString(`WORK="$HOME/dd-installer-hyperv-fix"
rm -rf "$WORK"; mkdir -p "$WORK"; cd "$WORK"
command -v wget >/dev/null 2>&1 || command -v curl >/dev/null 2>&1 || sudo apk add wget
if command -v wget >/dev/null 2>&1; then wget -O fix.tar.gz ` + install.Shq(url) + `; else curl -fL -o fix.tar.gz ` + install.Shq(url) + `; fi
tar -xzf fix.tar.gz
cd */ || { echo "unexpected archive layout" >&2; exit 1; }
[ -f install.sh ] || { echo "install.sh missing in the archive" >&2; exit 1; }
chmod +x install.sh
./install.sh install
`)
	} else {
		b.WriteString(`cd "$HOME/dd-installer-hyperv-fix"/*/ 2>/dev/null || { echo "The fix has not been installed by DD-Installer yet." >&2; exit 1; }
./install.sh status
`)
	}
	return b.String()
}
