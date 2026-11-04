// Package assistant is the installer's automatic problem solver.
//
// When a step fails, the failing output (plus the probed host facts) is
// matched against a rule base derived from the official Dune Docker
// documentation (docs.dunedocker.app, source: Red-Blink/dune-docker-docs) and
// the upstream installer's own messages. A rule explains the problem and may
// carry fix actions:
//
//   - RiskSafe:     idempotent, local to the install, applied automatically
//   - RiskModerate: changes the host (groups, sudoers, packages); the user is
//     asked first
//   - RiskManual:   cannot be done by the installer; instructions only
//
// The assistant never runs destructive actions (deleting data, pruning
// volumes, formatting disks). Each fix runs at most once per rule per install.
package assistant

import (
	"regexp"
	"slices"
	"strings"

	"github.com/dev-prophet-code/dd-installer/internal/probe"
)

type Risk string

const (
	RiskSafe     Risk = "safe"
	RiskModerate Risk = "moderate"
	RiskManual   Risk = "manual"
)

// Action is one remediation step.
type Action struct {
	Description string `json:"description"`
	Script      string `json:"-"` // POSIX sh; empty for manual guidance
	// Retry says the failed step should be re-run after this action succeeds.
	Retry bool `json:"retry"`
	// Wait pauses (seconds) before the retry, e.g. for transient outages.
	Wait int `json:"wait,omitempty"`
	// Reconnect asks the runner to open a fresh session first (new group
	// membership only applies to new logins).
	Reconnect bool `json:"reconnect,omitempty"`
	// Patch asks the orchestrator to adjust the plan (see PlanPatch).
	Patch *PlanPatch `json:"patch,omitempty"`
}

// PlanPatch lets a rule change the plan before a retry.
type PlanPatch struct {
	NextFreeWebPort      bool `json:"nextFreeWebPort,omitempty"`
	ReuseExistingInstall bool `json:"reuseExistingInstall,omitempty"`
}

// Rule is one known problem.
type Rule struct {
	ID      string
	Title   string
	Steps   []string // restrict to these step IDs; empty = any
	Pattern *regexp.Regexp
	// When adds host-based conditions (nil = pattern only).
	When    func(Context) bool
	Explain string
	Risk    Risk
	Actions func(Context) []Action
	DocURL  string
	// MaxUses caps how often the fix may run per install (default 1);
	// transient problems (locks, outages, rate limits) allow several.
	MaxUses int
}

// Context is what a rule sees.
type Context struct {
	Step     string
	Output   string // redacted tail of the failed step
	ExitCode int
	Err      string
	Host     probe.Info
	Dir      string // install directory expression (shell-safe, double-quoted use)
	User     string
	WebPort  int
	SudoPass bool // an SSH/sudo password is available to the installer
}

// Diagnosis is a matched rule instantiated for a failure.
type Diagnosis struct {
	RuleID  string   `json:"ruleId"`
	Title   string   `json:"title"`
	Explain string   `json:"explain"`
	Risk    Risk     `json:"risk"`
	Actions []Action `json:"actions"`
	DocURL  string   `json:"docUrl,omitempty"`
	MaxUses int      `json:"-"`
}

// Diagnose returns all matching rules, most specific (first declared) first.
func Diagnose(c Context) []Diagnosis {
	hay := c.Output + "\n" + c.Err
	var out []Diagnosis
	for _, r := range Rules() {
		if len(r.Steps) > 0 && !slices.Contains(r.Steps, c.Step) {
			continue
		}
		if r.Pattern != nil && !r.Pattern.MatchString(hay) {
			continue
		}
		if r.When != nil && !r.When(c) {
			continue
		}
		d := Diagnosis{RuleID: r.ID, Title: r.Title, Explain: r.Explain, Risk: r.Risk, DocURL: r.DocURL, MaxUses: max(r.MaxUses, 1)}
		if r.Actions != nil {
			d.Actions = r.Actions(c)
		}
		out = append(out, d)
	}
	return out
}

// Tail returns the last n lines of s.
func Tail(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}
