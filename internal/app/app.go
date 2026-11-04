// Package app is the UI-independent controller of the installer. A native UI
// (internal/ui) collects a Request, calls Scan/Start and renders the events it
// receives. Nothing here touches the disk; secrets live in memory only.
package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/dev-prophet-code/dd-installer/internal/assistant"
	"github.com/dev-prophet-code/dd-installer/internal/install"
	"github.com/dev-prophet-code/dd-installer/internal/model"
	"github.com/dev-prophet-code/dd-installer/internal/platform"
	"github.com/dev-prophet-code/dd-installer/internal/probe"
	"github.com/dev-prophet-code/dd-installer/internal/sizing"
	"github.com/dev-prophet-code/dd-installer/internal/transport"
)

// Version is set at build time.
var Version = "0.0.2-alpha" // overridden at build time from the VERSION file

// Request is everything the user entered, including secrets.
type Request struct {
	Config             model.Config
	SSHPassword        string
	SSHPassphrase      string
	FuncomToken        string
	ExperimentalCombat bool
	AcceptDisclaimer   bool
}

// Cfg returns the config with secrets applied.
func (r Request) Cfg() model.Config {
	c := r.Config
	c.SSH.Password = r.SSHPassword
	c.SSH.Passphrase = r.SSHPassphrase
	c.FuncomToken = strings.TrimSpace(r.FuncomToken)
	return c
}

// ScanResult is the outcome of a host scan.
type ScanResult struct {
	Target          string
	Host            probe.Info
	Report          sizing.Report
	Err             string
	Diagnoses       []assistant.Diagnosis
	HostKeyRequired bool
	Fingerprint     string
	KeyMismatch     bool
}

// Controller coordinates scans and installations.
type Controller struct {
	pf platform.Platform

	mu      sync.Mutex
	running bool
	cancel  context.CancelFunc
	events  []install.Event
	subs    map[chan struct{}]struct{}
	confirm map[string]chan bool
	seq     int
}

// New creates a controller for the given platform.
func New(pf platform.Platform) *Controller {
	return &Controller{pf: pf, subs: map[chan struct{}]struct{}{}, confirm: map[string]chan bool{}}
}

// Platform returns the platform.
func (c *Controller) Platform() platform.Platform { return c.pf }

// Validate checks a request without connecting anywhere.
func (c *Controller) Validate(r Request) []string { cfg := r.Cfg(); return cfg.Validate() }

// HostKey fetches the SSH host-key fingerprint for confirmation.
func (c *Controller) HostKey(ctx context.Context, host string, port int) (string, error) {
	if strings.TrimSpace(host) == "" || port < 1 || port > 65535 {
		return "", errors.New("host and port required")
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	return transport.ScanHostKey(ctx, host, port)
}

func (c *Controller) connect(ctx context.Context, cfg model.Config) (transport.Runner, error) {
	if cfg.Target == model.TargetSSH {
		return transport.DialSSH(ctx, cfg.SSH)
	}
	if c.pf == nil {
		return nil, errors.New("no platform registered")
	}
	if ls := c.pf.LocalSupport(); !ls.Supported {
		return nil, errors.New(ls.Reason)
	}
	return c.pf.LocalRunner(cfg)
}

// Scan connects to the target and analyses it.
func (c *Controller) Scan(ctx context.Context, r Request) ScanResult {
	cfg := r.Cfg()
	for _, e := range cfg.Validate() { // only connection-related problems block a scan
		for _, p := range []string{"ssh:", "target", "installDir", "wslDistro"} {
			if strings.HasPrefix(e, p) {
				return ScanResult{Err: e}
			}
		}
	}
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	run, err := c.connect(ctx, cfg)
	if err != nil {
		var hk *transport.HostKeyError
		if errors.As(err, &hk) {
			return ScanResult{HostKeyRequired: true, Fingerprint: hk.Fingerprint, KeyMismatch: hk.Mismatch, Err: hk.Error()}
		}
		msg := Sanitize(err.Error(), cfg)
		return ScanResult{Err: msg, Diagnoses: assistant.Diagnose(assistant.Context{Step: "connect", Err: msg})}
	}
	defer run.Close()
	out, code, err := transport.Capture(ctx, run, install.ProbeScript(cfg))
	if err != nil || code != 0 {
		msg := "scan failed"
		if err != nil {
			msg = err.Error()
		}
		return ScanResult{Err: Sanitize(msg, cfg)}
	}
	info := probe.Parse(out)
	info.Raw = nil
	return ScanResult{Target: run.Describe(), Host: info, Report: sizing.Analyse(info, cfg)}
}

// Sanitize strips secrets from error strings.
func Sanitize(s string, c model.Config) string {
	for _, sec := range []string{c.SSH.Password, c.SSH.Passphrase, c.FuncomToken} {
		if len(sec) >= 4 {
			s = strings.ReplaceAll(s, sec, "***")
		}
	}
	return s
}

// ProvisionLocal installs missing local prerequisites (Windows: WSL2+Ubuntu).
func (c *Controller) ProvisionLocal(ctx context.Context, log func(string)) (bool, error) {
	if c.pf == nil {
		return false, errors.New("no platform registered")
	}
	return c.pf.ProvisionLocal(ctx, log)
}

// Running reports whether an installation is in progress.
func (c *Controller) Running() bool { c.mu.Lock(); defer c.mu.Unlock(); return c.running }

// Subscribe delivers all past and future events to fn (in order, from its own
// goroutine). Call the returned function to stop.
func (c *Controller) Subscribe(fn func(install.Event)) (stop func()) {
	wake := make(chan struct{}, 1)
	wake <- struct{}{}
	done := make(chan struct{})
	c.mu.Lock()
	c.subs[wake] = struct{}{}
	c.mu.Unlock()
	go func() {
		idx := 0
		for {
			select {
			case <-wake:
				c.mu.Lock()
				if idx > len(c.events) { // a new run reset the history
					idx = 0
				}
				batch := append([]install.Event(nil), c.events[idx:]...)
				idx = len(c.events)
				c.mu.Unlock()
				for _, e := range batch {
					fn(e)
				}
			case <-done:
				return
			}
		}
	}()
	return func() {
		c.mu.Lock()
		delete(c.subs, wake)
		c.mu.Unlock()
		close(done)
	}
}

func (c *Controller) emit(e install.Event) {
	c.mu.Lock()
	defer c.mu.Unlock()
	// Log lines are capped; structural events are always kept.
	if e.Type != "log" || len(c.events) < 50000 {
		c.events = append(c.events, e)
	}
	for ch := range c.subs {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

// Start begins an installation in the background.
func (c *Controller) Start(r Request) error {
	if !r.AcceptDisclaimer {
		return errors.New("you must accept the disclaimer before installing")
	}
	cfg := r.Cfg()
	if errs := cfg.Validate(); len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	if cfg.FuncomToken == "" {
		return errors.New("Funcom token is required")
	}
	c.mu.Lock()
	if c.running {
		c.mu.Unlock()
		return errors.New("an installation is already running")
	}
	c.running = true
	c.events = nil
	ctx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel
	c.mu.Unlock()

	go func() {
		defer func() {
			c.mu.Lock()
			c.running = false
			c.cancel = nil
			c.mu.Unlock()
			cancel()
		}()
		run, err := c.connect(ctx, cfg)
		if err != nil {
			c.emit(install.Event{Type: "error", Msg: Sanitize(err.Error(), cfg)})
			return
		}
		out, _, err := transport.Capture(ctx, run, install.ProbeScript(cfg))
		if err != nil {
			c.emit(install.Event{Type: "error", Msg: Sanitize(err.Error(), cfg)})
			run.Close()
			return
		}
		info := probe.Parse(out)
		rep := sizing.Analyse(info, cfg)
		if !rep.OK {
			for _, f := range rep.Findings {
				if f.Level == sizing.Error {
					c.emit(install.Event{Type: "error", Msg: f.Message})
				}
			}
			run.Close()
			return
		}
		o := install.Options{
			Cfg: cfg, Runner: run, Host: info, Report: rep, Emit: c.emit,
			ExperimentalCombat: r.ExperimentalCombat,
			Confirm:            c.askConfirm,
			Reconnect:          func(ctx context.Context) (transport.Runner, error) { return c.connect(ctx, cfg) },
		}
		if err := install.Run(ctx, o); err != nil { // Run closes the runner
			c.emit(install.Event{Type: "error", Msg: Sanitize(err.Error(), cfg)})
		}
	}()
	return nil
}

func (c *Controller) askConfirm(ctx context.Context, id string, d assistant.Diagnosis) bool {
	c.mu.Lock()
	c.seq++
	key := fmt.Sprintf("%s#%d", id, c.seq)
	ch := make(chan bool, 1)
	c.confirm[key] = ch
	c.mu.Unlock()
	c.emit(install.Event{Type: "confirm", ConfirmID: key, Diagnosis: &d})
	select {
	case v := <-ch:
		return v
	case <-ctx.Done():
		return false
	}
}

// Confirm answers a pending assistant question.
func (c *Controller) Confirm(id string, approve bool) {
	c.mu.Lock()
	ch, ok := c.confirm[id]
	delete(c.confirm, id)
	c.mu.Unlock()
	if ok {
		ch <- approve
	}
}

// Cancel stops the running installation.
func (c *Controller) Cancel() {
	c.mu.Lock()
	if c.cancel != nil {
		c.cancel()
	}
	c.mu.Unlock()
}
