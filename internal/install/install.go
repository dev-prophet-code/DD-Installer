// Package install orchestrates the installation on a target and drives the
// problem-solving assistant when a step fails.
package install

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/dev-prophet-code/dd-installer/internal/assistant"
	"github.com/dev-prophet-code/dd-installer/internal/model"
	"github.com/dev-prophet-code/dd-installer/internal/probe"
	"github.com/dev-prophet-code/dd-installer/internal/sizing"
	"github.com/dev-prophet-code/dd-installer/internal/transport"
)

// Event is streamed to the UI.
type Event struct {
	Type string `json:"type"` // step | log | diagnosis | fix | confirm | done | error
	Step string `json:"step,omitempty"`
	// Status for type "step": running | ok | failed | skipped
	Status    string               `json:"status,omitempty"`
	Msg       string               `json:"msg,omitempty"`
	Diagnosis *assistant.Diagnosis `json:"diagnosis,omitempty"`
	ConfirmID string               `json:"confirmId,omitempty"`
}

// Options wires an installation run.
type Options struct {
	Cfg    model.Config
	Runner transport.Runner
	Host   probe.Info
	Report sizing.Report
	Emit   func(Event)
	// Confirm asks the user to approve a moderate-risk fix. It must return
	// false when the context ends.
	Confirm func(ctx context.Context, id string, d assistant.Diagnosis) bool
	// Reconnect opens a fresh session (new group membership). Optional.
	Reconnect func(ctx context.Context) (transport.Runner, error)
	// ExperimentalCombat also sets per-Sietch PvP/PvE selectors.
	ExperimentalCombat bool
	// MaxFixAttempts per step (default 3).
	MaxFixAttempts int
	// Sleep allows tests to skip waits.
	Sleep func(time.Duration)
}

type step struct {
	ID, Title string
	Script    func(*state) string
	Optional  bool // failure does not abort the run
}

type state struct {
	port      int
	reuse     bool
	sudoGrant bool
}

// Run executes the full installation. It returns nil on success.
func Run(ctx context.Context, o Options) error {
	if o.Sleep == nil {
		o.Sleep = time.Sleep
	}
	if o.MaxFixAttempts == 0 {
		o.MaxFixAttempts = 3
	}
	st := &state{port: o.Cfg.WebPort}
	red := newRedactor(o.Cfg.FuncomToken, o.Cfg.SSH.Password, o.Cfg.SSH.Passphrase)
	secrets := o.Cfg.SSH.Password

	steps := []step{
		{"bootstrap", "Download Dune Docker", func(s *state) string { return bootstrapScript(o.Cfg, s.reuse) }, false},
		{"install", "Install Docker & start the Web UI", func(s *state) string { return installScript(o.Cfg, s.port) }, false},
		{"tuning", "Apply performance tuning", func(s *state) string {
			if !o.Cfg.ApplyTuning {
				return ""
			}
			return tuningScript(o.Cfg, o.Report.Env)
		}, true},
		{"console-setup", "Configure server & deploy game files", func(s *state) string {
			return consoleScript(o.Cfg, "setup", mustJSON(map[string]any{
				"port": s.port, "passwordFile": passwordFile(o.Cfg),
				"title": o.Cfg.ServerTitle, "region": o.Cfg.Region, "token": o.Cfg.FuncomToken,
			}))
		}, false},
		{"start", "Start the battlegroup", func(s *state) string {
			return consoleScript(o.Cfg, "start", mustJSON(map[string]any{"port": s.port, "passwordFile": passwordFile(o.Cfg)}))
		}, false},
		{"ready", "Wait for readiness", func(s *state) string { return readyScript(o.Cfg, 45) }, false},
		{"layout", "Create Sietches & Deep Desert", func(s *state) string {
			return consoleScript(o.Cfg, "layout", mustJSON(map[string]any{
				"port": s.port, "passwordFile": passwordFile(o.Cfg),
				"sietches": o.Cfg.Sietches, "deepDesert": o.Cfg.DeepDesert, "combat": o.ExperimentalCombat,
			}))
		}, false},
		{"post", "Final tuning commands", func(s *state) string {
			if !o.Cfg.ApplyTuning {
				return ""
			}
			return postScript(o.Cfg, o.Report.PostCommands)
		}, true},
	}

	defer func() {
		if st.sudoGrant {
			c, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			_, _ = o.Runner.Run(c, sudoRevokeScript, io.Discard)
			o.Emit(Event{Type: "log", Msg: "Temporary sudo grant removed."})
		}
	}()

	defer func() { _ = o.Runner.Close() }() // Run owns the runner (it may be replaced on reconnect)
	fixed := map[string]int{}               // rule -> times applied; capped by the rule's MaxUses
	for _, s := range steps {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if s.Script(st) == "" {
			o.Emit(Event{Type: "step", Step: s.ID, Status: "skipped", Msg: s.Title})
			continue
		}
		o.Emit(Event{Type: "step", Step: s.ID, Status: "running", Msg: s.Title})
		attempts := 0
		for {
			script := s.Script(st)
			code, out, err := runScript(ctx, o, red, script)
			if err == nil && code == 0 {
				o.Emit(Event{Type: "step", Step: s.ID, Status: "ok", Msg: s.Title})
				break
			}
			errStr := ""
			if err != nil {
				errStr = err.Error()
			}
			o.Emit(Event{Type: "step", Step: s.ID, Status: "failed", Msg: fmt.Sprintf("%s (exit %d) %s", s.Title, code, red.apply(errStr))})
			if attempts >= o.MaxFixAttempts || ctx.Err() != nil {
				if s.Optional {
					o.Emit(Event{Type: "log", Msg: "Optional step failed; continuing."})
					break
				}
				return fmt.Errorf("step %q failed (exit %d)", s.ID, code)
			}
			retried, abort := solve(ctx, &o, st, s.ID, red.apply(assistant.Tail(out, 60)), red.apply(errStr), code, fixed, secrets)
			if abort {
				return fmt.Errorf("step %q failed and needs manual action", s.ID)
			}
			if !retried {
				if s.Optional {
					break
				}
				return fmt.Errorf("step %q failed (exit %d)", s.ID, code)
			}
			attempts++
			o.Emit(Event{Type: "step", Step: s.ID, Status: "running", Msg: s.Title + " (retry " + fmt.Sprint(attempts) + ")"})
		}
	}
	o.Emit(Event{Type: "done", Msg: "Installation finished."})
	return nil
}

// solve consults the assistant; returns whether the step should be retried.
func solve(ctx context.Context, o *Options, st *state, stepID, tail, errStr string, code int, fixed map[string]int, sudoPW string) (retry bool, abort bool) {
	host := o.Host
	diags := assistant.Diagnose(assistant.Context{
		Step: stepID, Output: tail, ExitCode: code, Err: errStr, Host: host,
		Dir: dirExpr(o.Cfg), User: sshUser(o.Cfg, host), WebPort: st.port, SudoPass: sudoPW != "",
	})
	if len(diags) == 0 {
		o.Emit(Event{Type: "diagnosis", Step: stepID, Diagnosis: &assistant.Diagnosis{
			RuleID: "unknown", Title: "No known fix for this error", Risk: assistant.RiskManual,
			Explain: "The assistant has no rule for this failure. Copy the log above and check " +
				"https://docs.dunedocker.app/operations/troubleshooting or the Discord community. " +
				"Run `runtime/scripts/dune doctor` on the server for a health report.",
			DocURL: "https://docs.dunedocker.app/operations/troubleshooting"}})
		return false, false
	}
	manual := false
	for i := range diags {
		d := diags[i]
		o.Emit(Event{Type: "diagnosis", Step: stepID, Diagnosis: &d})
		if fixed[d.RuleID] >= d.MaxUses {
			o.Emit(Event{Type: "log", Msg: "Fix for \"" + d.Title + "\" was already tried; not repeating it."})
			continue
		}
		if d.Risk == assistant.RiskManual {
			manual = true // instructions only; other matching rules may still be fixable
			continue
		}
		if d.Risk == assistant.RiskModerate {
			if o.Confirm == nil || !o.Confirm(ctx, d.RuleID, d) {
				o.Emit(Event{Type: "log", Msg: "Fix declined: " + d.Title})
				return false, true
			}
		}
		fixed[d.RuleID]++
		ok := true
		for _, a := range d.Actions {
			if a.Script != "" {
				o.Emit(Event{Type: "fix", Step: stepID, Msg: a.Description})
				script := expand(a.Script, sshUser(o.Cfg, host))
				if strings.Contains(a.Script, "@sudo-temp-grant") {
					script = "DD_SUDO_PW=" + shq(sudoPW) + "\nexport DD_SUDO_PW\n" + script
					st.sudoGrant = true
				}
				c, _, err := runScript(ctx, *o, newRedactor(o.Cfg.FuncomToken, sudoPW), withPrelude(script))
				if err != nil || c != 0 {
					o.Emit(Event{Type: "log", Msg: fmt.Sprintf("Fix action failed (exit %d)", c)})
					ok = false
					break
				}
			} else {
				o.Emit(Event{Type: "fix", Step: stepID, Msg: a.Description})
			}
			if a.Patch != nil {
				if a.Patch.NextFreeWebPort {
					st.port = nextPort(st.port)
					o.Emit(Event{Type: "log", Msg: fmt.Sprintf("Using Web UI port %d.", st.port)})
				}
				if a.Patch.ReuseExistingInstall {
					st.reuse = true
				}
			}
			if a.Reconnect && o.Reconnect != nil {
				if r, err := o.Reconnect(ctx); err == nil {
					_ = o.Runner.Close()
					o.Runner = r
				} else {
					o.Emit(Event{Type: "log", Msg: "Reconnect failed: " + err.Error()})
					ok = false
					break
				}
			}
			if a.Wait > 0 {
				o.Emit(Event{Type: "log", Msg: fmt.Sprintf("Waiting %d s...", a.Wait)})
				o.Sleep(time.Duration(a.Wait) * time.Second)
			}
			if a.Retry {
				retry = true
			}
		}
		if ok && retry {
			return true, false
		}
		if !ok {
			return false, true
		}
	}
	return retry, manual && !retry
}

func nextPort(p int) int {
	if p >= 65000 {
		return 8089
	}
	return p + 1
}

func passwordFile(cfg model.Config) string {
	return dirExpr(cfg) + "/runtime/secrets/admin-web-password.txt"
}

func sshUser(cfg model.Config, h probe.Info) string {
	if cfg.Target == model.TargetSSH && cfg.SSH.User != "" {
		return cfg.SSH.User
	}
	return h.User
}

// runScript executes script on the runner, streaming redacted lines to the
// UI and returning the redacted output.
func runScript(ctx context.Context, o Options, red *redactor, script string) (int, string, error) {
	var mu sync.Mutex
	var all bytes.Buffer
	w := &lineWriter{fn: func(line string) {
		line = red.line(line)
		mu.Lock()
		all.WriteString(line + "\n")
		mu.Unlock()
		o.Emit(Event{Type: "log", Msg: line})
	}}
	code, err := o.Runner.Run(ctx, script, w)
	w.Flush()
	mu.Lock()
	defer mu.Unlock()
	return code, all.String(), err
}

func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(b)
}

// shq single-quotes s for POSIX sh.
func shq(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }
