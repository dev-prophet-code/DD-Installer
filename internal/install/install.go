// Package install orchestrates the installation on a target and drives the
// problem-solving assistant when a step fails.
package install

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
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
	Port      int                  `json:"port,omitempty"` // web console port, set on "done"
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
	// Upstream returns the current checksum list (default: FetchManifest).
	Upstream func(context.Context) (Manifest, error)
}

type step struct {
	ID, Title string
	Script    func(*state) string
	// Check, if set, returns a quick script that exits 0 when the step is
	// already done on the target; the step is then skipped (resume support).
	Check func(*state) string
	// Detach runs the step in the background on the target and follows its
	// log, so a dropped connection or a sleeping laptop cannot stop it.
	Detach   bool
	Optional bool // failure does not abort the run
}

type state struct {
	port      int
	pin       Pin    // the release to download and its expected checksum
	testedTag string // the release DD-Installer was verified with
	reuse     bool
	sudoGrant bool
	version   string // upstream version found on the target
}

var (
	portRe    = regexp.MustCompile(`(?m)^DD_PORT (\d{1,5})\s*$`)
	versionRe = regexp.MustCompile(`(?m)^DD_UPSTREAM_VERSION (\S+)\s*$`)
)

// learn reads marker lines printed by step and check scripts.
func (st *state) learn(out string) {
	if m := portRe.FindAllStringSubmatch(out, -1); len(m) > 0 {
		if p, err := strconv.Atoi(m[len(m)-1][1]); err == nil && p > 0 && p < 65536 {
			st.port = p
		}
	}
	if m := versionRe.FindAllStringSubmatch(out, -1); len(m) > 0 {
		st.version = m[len(m)-1][1]
	}
}

func buildSteps(o *Options) []step {
	cfg := o.Cfg
	setupParams := func(s *state, extra map[string]any) string {
		m := map[string]any{"port": s.port, "passwordFile": passwordFile(cfg)}
		for k, v := range extra {
			m[k] = v
		}
		return mustJSON(m)
	}
	readyCheck := func(*state) string {
		return withPrelude(fmt.Sprintf("cd \"%s\" 2>/dev/null || exit 1\ndgrp runtime/scripts/dune ready >/dev/null 2>&1\n", dirExpr(cfg)))
	}
	return []step{
		{ID: "bootstrap", Title: "Download Dune Docker",
			Script: func(s *state) string { return pinnedBootstrapScript(cfg, s.pin) },
			Check:  func(*state) string { return checkBootstrapScript(cfg) }},
		{ID: "install", Title: "Install Docker & start the Web UI", Detach: true,
			Script: func(s *state) string { return installScript(cfg, s.port) },
			Check:  func(s *state) string { return checkInstallScript(cfg, s.port) }},
		{ID: "tuning", Title: "Apply performance tuning", Optional: true,
			Script: func(*state) string {
				if !cfg.ApplyTuning {
					return ""
				}
				return tuningScript(cfg, o.Report.Env)
			}},
		{ID: "console-setup", Title: "Configure server & deploy game files",
			Script: func(s *state) string {
				return consoleScript(cfg, "setup", setupParams(s, map[string]any{"title": cfg.ServerTitle, "region": cfg.Region, "token": cfg.FuncomToken}))
			},
			Check: func(s *state) string { return consoleScript(cfg, "check-setup", setupParams(s, nil)) }},
		{ID: "start", Title: "Start the battlegroup",
			Script: func(s *state) string { return consoleScript(cfg, "start", setupParams(s, nil)) },
			Check:  readyCheck},
		{ID: "ready", Title: "Wait for readiness",
			Script: func(*state) string { return readyScript(cfg, 45) },
			Check:  readyCheck},
		{ID: "layout", Title: "Create Sietches & Deep Desert",
			Script: func(s *state) string {
				return consoleScript(cfg, "layout", setupParams(s, map[string]any{"sietches": cfg.Sietches, "deepDesert": cfg.DeepDesert, "combat": o.ExperimentalCombat}))
			}},
		{ID: "verify", Title: "Verify the resulting layout", Optional: true,
			Script: func(*state) string { return verifyScript(cfg) }},
		{ID: "firewall", Title: "Open game ports in the server firewall", Optional: true,
			Script: func(s *state) string {
				if !cfg.ConfigureFirewall || cfg.Target != model.TargetSSH {
					return "" // a local/WSL install has no server firewall to configure
				}
				return firewallScript(cfg, s.port)
			}},
		{ID: "post", Title: "Final tuning commands", Optional: true,
			Script: func(*state) string {
				if !cfg.ApplyTuning {
					return ""
				}
				return postScript(cfg, o.Report.PostCommands)
			}},
		// Last step: prove that the installation is complete and works. Right
		// after it the temporary sudo access is locked again (see Run).
		{ID: "finalize", Title: "Verify the finished installation", Optional: true,
			Script: func(s *state) string { return completeCheckScript(cfg, s.port) }},
	}
}

// lockSudo removes the temporary sudo grant and VERIFIES that passwordless sudo
// is really gone. It is a no-op when no grant was made. Called right after the
// installation has been verified, and again from a defer as a safety net.
func (o *Options) lockSudo(st *state) {
	if !st.sudoGrant || o.Runner == nil {
		return
	}
	c, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	_, _, _ = transport.Capture(c, o.Runner, sudoRevokeScript)
	if _, code, err := transport.Capture(c, o.Runner, sudoRevokedCheckScript); err == nil && code == 0 {
		st.sudoGrant = false
		o.Emit(Event{Type: "log", Msg: "Temporary sudo access removed and verified: sudo asks for a password again."})
		return
	}
	o.Emit(Event{Type: "log", Msg: "WARNING: DD-Installer could not confirm that the temporary sudo access is gone. Remove it yourself on the server: sudo rm -f /etc/sudoers.d/90-dd-installer"})
}

// Run executes the full installation. It returns nil on success. Running it
// again after a failure continues where it stopped: finished steps are
// detected on the target and skipped.
func Run(ctx context.Context, o Options) error {
	if o.Sleep == nil {
		o.Sleep = time.Sleep
	}
	if o.MaxFixAttempts == 0 {
		o.MaxFixAttempts = 3
	}
	st := &state{port: o.Cfg.WebPort}
	// Read the CURRENT expected checksum before anything is downloaded.
	pin, testedTag, note, perr := ResolvePin(ctx, o.Cfg.UpstreamLatest, o.Upstream)
	if perr != nil {
		return perr
	}
	st.pin, st.testedTag = pin, testedTag
	o.Emit(Event{Type: "log", Msg: note})
	red := newRedactor(o.Cfg.FuncomToken, o.Cfg.SSH.Password, o.Cfg.SSH.Passphrase)
	secrets := o.Cfg.SSH.Password
	steps := buildSteps(&o)

	defer func() { o.lockSudo(st) }()       // safety net for failures and cancellation
	defer func() { _ = o.Runner.Close() }() // Run owns the runner (it may be replaced on reconnect)

	fixed := map[string]int{} // rule -> times applied; capped by the rule's MaxUses
	for _, s := range steps {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if s.Script(st) == "" {
			o.Emit(Event{Type: "step", Step: s.ID, Status: "skipped", Msg: s.Title})
			continue
		}
		if s.Check != nil {
			if out, code, err := transport.Capture(ctx, o.Runner, s.Check(st)); err == nil && code == 0 {
				st.learn(out)
				o.Emit(Event{Type: "step", Step: s.ID, Status: "skipped", Msg: s.Title + " (already done)"})
				continue
			}
		}
		o.Emit(Event{Type: "step", Step: s.ID, Status: "running", Msg: s.Title})
		attempts, lost := 0, 0
		for {
			script := s.Script(st)
			var (
				code int
				out  string
				err  error
			)
			if s.Detach {
				code, out, err = runDetached(ctx, &o, red, s.ID, script)
			} else {
				code, out, err = runScript(ctx, o, red, script)
			}
			st.learn(out)
			if err == nil && code == 0 {
				o.Emit(Event{Type: "step", Step: s.ID, Status: "ok", Msg: s.Title})
				if s.ID == "install" && st.version != "" && !tested(st.version, st.testedTag) {
					o.Emit(Event{Type: "log", Msg: fmt.Sprintf("WARNING: the server runs Dune Docker %s but DD-Installer was tested with %s. Layout steps may not work with other versions.", st.version, st.testedTag)})
				}
				break
			}
			if err != nil && transport.IsConnectionLost(err) && lost < 5 && ctx.Err() == nil {
				lost++
				if o.reconnect(ctx) {
					o.Emit(Event{Type: "log", Msg: "Continuing the step after reconnecting (it is safe to repeat)."})
					continue
				}
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
				return fmt.Errorf("step %q failed (exit %d). Press Install again to continue from here", s.ID, code)
			}
			retried, abort := solve(ctx, &o, st, s.ID, red.apply(assistant.Tail(out, 60)), red.apply(errStr), code, fixed, secrets)
			if abort {
				return fmt.Errorf("step %q failed and needs manual action. After fixing it, press Install again to continue", s.ID)
			}
			if !retried {
				if s.Optional {
					break
				}
				return fmt.Errorf("step %q failed (exit %d). Press Install again to continue from here", s.ID, code)
			}
			attempts++
			o.Emit(Event{Type: "step", Step: s.ID, Status: "running", Msg: s.Title + " (retry " + fmt.Sprint(attempts) + ")"})
		}
	}
	// Everything is installed and was verified by the "finalize" step: lock the
	// temporary sudo access NOW, not only when the window is closed.
	o.lockSudo(st)
	o.Emit(Event{Type: "done", Msg: "Installation finished.", Port: st.port})
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

// lineSink turns raw output into redacted log events and keeps a copy.
type lineSink struct {
	w   *lineWriter
	mu  sync.Mutex
	all *bytes.Buffer
}

func newLineSink(o *Options, red *redactor, all *bytes.Buffer) *lineSink {
	s := &lineSink{all: all}
	s.w = &lineWriter{fn: func(line string) {
		line = red.line(line)
		s.mu.Lock()
		s.all.WriteString(line + "\n")
		s.mu.Unlock()
		if strings.HasPrefix(line, "DD_PORT ") || strings.HasPrefix(line, "DD_UPSTREAM_VERSION ") || strings.HasPrefix(line, "DD_DETACH ") {
			return // machine-readable markers, not for the user
		}
		o.Emit(Event{Type: "log", Msg: line})
	}}
	return s
}

func (s *lineSink) Write(p []byte) (int, error) { return s.w.Write(p) }
func (s *lineSink) Flush()                      { s.w.Flush() }

// runScript executes script on the runner, streaming redacted lines to the
// UI and returning the redacted output.
func runScript(ctx context.Context, o Options, red *redactor, script string) (int, string, error) {
	var all bytes.Buffer
	sink := newLineSink(&o, red, &all)
	code, err := o.Runner.Run(ctx, script, sink)
	sink.Flush()
	sink.mu.Lock()
	defer sink.mu.Unlock()
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
