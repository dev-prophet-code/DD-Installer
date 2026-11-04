// Package gui serves the installer's graphical interface: a small local web
// application (embedded HTML/JS) shown in the user's browser or an app window.
//
// Security model: the server binds to 127.0.0.1 only, requires a random
// per-launch token (cookie + header), validates Host and Origin to defeat DNS
// rebinding and cross-site requests, accepts only JSON POSTs, and keeps all
// secrets (SSH password, Funcom token) in memory for the current run.
package gui

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/url"
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

//go:embed web
var webFS embed.FS

// Version is set at build time.
var Version = "0.1.0-draft"

const maxBody = 1 << 20

// Server is the GUI backend.
type Server struct {
	token string
	addr  string
	mux   *http.ServeMux
	pf    platform.Platform

	mu      sync.Mutex
	running bool
	cancel  context.CancelFunc
	events  []install.Event
	subs    map[chan struct{}]struct{}
	confirm map[string]chan bool
}

// New creates a server. The platform may be nil only in tests.
func New(pf platform.Platform) *Server {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	s := &Server{token: hex.EncodeToString(b), pf: pf, subs: map[chan struct{}]struct{}{}, confirm: map[string]chan bool{}}
	s.routes()
	return s
}

// Listen binds a random loopback port and returns the launch URL.
func (s *Server) Listen() (net.Listener, string, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, "", err
	}
	s.addr = ln.Addr().String()
	return ln, "http://" + s.addr + "/?t=" + s.token, nil
}

// Serve runs until ctx ends.
func (s *Server) Serve(ctx context.Context, ln net.Listener) error {
	srv := &http.Server{Handler: s.mux, ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 2 * time.Minute}
	go func() {
		<-ctx.Done()
		c, cc := context.WithTimeout(context.Background(), 3*time.Second)
		defer cc()
		_ = srv.Shutdown(c)
	}()
	if err := srv.Serve(ln); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// Handler exposes the router (tests).
func (s *Server) Handler() http.Handler { return s.mux }

// Token exposes the launch token (tests).
func (s *Server) Token() string { return s.token }

// SetAddr sets the expected host:port (tests using httptest).
func (s *Server) SetAddr(a string) { s.addr = a }

func (s *Server) routes() {
	m := http.NewServeMux()
	sub, _ := fs.Sub(webFS, "web")
	static := http.FileServer(http.FS(sub))
	m.Handle("/", s.guard(false, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" && r.URL.Query().Get("t") != "" {
			// Exchange the URL token for an HttpOnly cookie, then drop it from the URL.
			if !s.tokenEq(r.URL.Query().Get("t")) {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			http.SetCookie(w, &http.Cookie{Name: "dd_token", Value: s.token, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode})
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		static.ServeHTTP(w, r)
	}))
	m.HandleFunc("/api/env", s.guard(true, s.handleEnv))
	m.HandleFunc("/api/validate", s.guard(true, s.handleValidate))
	m.HandleFunc("/api/ssh/hostkey", s.guard(true, s.handleHostKey))
	m.HandleFunc("/api/scan", s.guard(true, s.handleScan))
	m.HandleFunc("/api/install", s.guard(true, s.handleInstall))
	m.HandleFunc("/api/events", s.guard(true, s.handleEvents))
	m.HandleFunc("/api/confirm", s.guard(true, s.handleConfirm))
	m.HandleFunc("/api/cancel", s.guard(true, s.handleCancel))
	s.mux = m
}

func (s *Server) tokenEq(t string) bool {
	return subtle.ConstantTimeCompare([]byte(t), []byte(s.token)) == 1
}

// guard applies Host/Origin/token/content-type checks and security headers.
func (s *Server) guard(needAuth bool, h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		hd := w.Header()
		hd.Set("X-Content-Type-Options", "nosniff")
		hd.Set("X-Frame-Options", "DENY")
		hd.Set("Referrer-Policy", "no-referrer")
		hd.Set("Cross-Origin-Resource-Policy", "same-origin")
		hd.Set("Content-Security-Policy", "default-src 'self'; style-src 'self'; script-src 'self'; connect-src 'self'; img-src 'self' data:; frame-ancestors 'none'; base-uri 'none'; form-action 'none'")

		if !s.hostOK(r.Host) {
			http.Error(w, "bad host", http.StatusForbidden) // DNS rebinding defence
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			if o := r.Header.Get("Origin"); o != "" {
				u, err := url.Parse(o)
				if err != nil || !s.hostOK(u.Host) {
					http.Error(w, "bad origin", http.StatusForbidden)
					return
				}
			}
			if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
				http.Error(w, "json required", http.StatusUnsupportedMediaType)
				return
			}
		}
		if needAuth {
			c, err := r.Cookie("dd_token")
			ok := err == nil && s.tokenEq(c.Value)
			if !ok && s.tokenEq(r.Header.Get("X-DD-Token")) {
				ok = true
			}
			if !ok {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxBody)
		h(w, r)
	}
}

func (s *Server) hostOK(h string) bool {
	if s.addr == "" {
		return false
	}
	_, port, err := net.SplitHostPort(s.addr)
	if err != nil {
		return false
	}
	return h == "127.0.0.1:"+port || h == "localhost:"+port
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

// request carries the config plus secrets (which Config never serialises).
type request struct {
	Config  model.Config `json:"config"`
	Secrets struct {
		SSHPassword   string `json:"sshPassword"`
		SSHPassphrase string `json:"sshPassphrase"`
		FuncomToken   string `json:"funcomToken"`
	} `json:"secrets"`
	ExperimentalCombat bool `json:"experimentalCombat"`
	// AcceptDisclaimer must be true to install (see docs/DISCLAIMER.md).
	AcceptDisclaimer bool `json:"acceptDisclaimer"`
}

func (rq *request) cfg() model.Config {
	c := rq.Config
	c.SSH.Password = rq.Secrets.SSHPassword
	c.SSH.Passphrase = rq.Secrets.SSHPassphrase
	c.FuncomToken = strings.TrimSpace(rq.Secrets.FuncomToken)
	return c
}

func decode(r *http.Request, v any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

func (s *Server) handleEnv(w http.ResponseWriter, r *http.Request) {
	out := map[string]any{"version": Version, "regions": model.Regions, "maxSietches": model.MaxSietches}
	if s.pf != nil {
		out["platform"] = s.pf.ID()
		out["platformName"] = s.pf.DisplayName()
		out["local"] = s.pf.LocalSupport()
		out["sshKeys"] = s.pf.DefaultSSHKeys()
	}
	writeJSON(w, 200, out)
}

func (s *Server) handleValidate(w http.ResponseWriter, r *http.Request) {
	var rq request
	if err := decode(r, &rq); err != nil {
		writeJSON(w, 400, map[string]any{"error": "invalid request"})
		return
	}
	c := rq.cfg()
	errs := c.Validate()
	writeJSON(w, 200, map[string]any{"errors": errs, "requiredRamGiB": sizing.RequiredRAM(c), "ddRoles": c.DeepDesert.DeepDesertRoles()})
}

func (s *Server) handleHostKey(w http.ResponseWriter, r *http.Request) {
	var rq struct {
		Host string `json:"host"`
		Port int    `json:"port"`
	}
	if err := decode(r, &rq); err != nil || rq.Host == "" || rq.Port < 1 || rq.Port > 65535 {
		writeJSON(w, 400, map[string]any{"error": "host and port required"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	fp, err := transport.ScanHostKey(ctx, rq.Host, rq.Port)
	if err != nil {
		writeJSON(w, 200, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]any{"fingerprint": fp})
}

// connect builds a runner for the configured target.
func (s *Server) connect(ctx context.Context, c model.Config) (transport.Runner, error) {
	if c.Target == model.TargetSSH {
		return transport.DialSSH(ctx, c.SSH)
	}
	if s.pf == nil {
		return nil, errors.New("no platform registered")
	}
	if ls := s.pf.LocalSupport(); !ls.Supported {
		return nil, errors.New(ls.Reason)
	}
	return s.pf.LocalRunner(c)
}

func (s *Server) handleScan(w http.ResponseWriter, r *http.Request) {
	var rq request
	if err := decode(r, &rq); err != nil {
		writeJSON(w, 400, map[string]any{"error": "invalid request"})
		return
	}
	c := rq.cfg()
	if errs := c.Validate(); len(errs) > 0 {
		// Server-side fields may still be incomplete during the scan; only
		// connection errors block here.
		for _, e := range errs {
			if strings.HasPrefix(e, "ssh:") || strings.HasPrefix(e, "target") || strings.HasPrefix(e, "installDir") || strings.HasPrefix(e, "wslDistro") {
				writeJSON(w, 200, map[string]any{"error": e})
				return
			}
		}
	}
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()
	run, err := s.connect(ctx, c)
	if err != nil {
		var hk *transport.HostKeyError
		if errors.As(err, &hk) {
			writeJSON(w, 200, map[string]any{"hostKeyRequired": true, "fingerprint": hk.Fingerprint, "mismatch": hk.Mismatch, "error": hk.Error()})
			return
		}
		writeJSON(w, 200, map[string]any{"error": sanitize(err.Error(), c), "diagnosis": diagnoseConnect(err.Error(), c)})
		return
	}
	defer run.Close()
	out, code, err := transport.Capture(ctx, run, install.ProbeScript(c))
	if err != nil || code != 0 {
		msg := fmt.Sprintf("scan failed (exit %d)", code)
		if err != nil {
			msg = err.Error()
		}
		writeJSON(w, 200, map[string]any{"error": sanitize(msg, c)})
		return
	}
	info := probe.Parse(out)
	info.Raw = nil
	rep := sizing.Analyse(info, c)
	writeJSON(w, 200, map[string]any{"target": run.Describe(), "host": info, "report": rep})
}

func diagnoseConnect(msg string, c model.Config) []assistant.Diagnosis {
	return assistant.Diagnose(assistant.Context{Step: "connect", Err: sanitize(msg, c)})
}

// sanitize strips secrets from error strings.
func sanitize(s string, c model.Config) string {
	for _, sec := range []string{c.SSH.Password, c.SSH.Passphrase, c.FuncomToken} {
		if len(sec) >= 4 {
			s = strings.ReplaceAll(s, sec, "***")
		}
	}
	return s
}

func (s *Server) emit(e install.Event) {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Log lines are capped; structural events (steps, diagnoses, done, error)
	// are always kept so a long log can never hide the outcome.
	if e.Type != "log" || len(s.events) < 50000 {
		s.events = append(s.events, e)
	}
	for ch := range s.subs {
		select {
		case ch <- struct{}{}:
		default: // a wake-up is already pending
		}
	}
}

func (s *Server) handleInstall(w http.ResponseWriter, r *http.Request) {
	var rq request
	if err := decode(r, &rq); err != nil {
		writeJSON(w, 400, map[string]any{"error": "invalid request"})
		return
	}
	if !rq.AcceptDisclaimer {
		writeJSON(w, 400, map[string]any{"errors": []string{"You must accept the disclaimer before installing."}})
		return
	}
	c := rq.cfg()
	if errs := c.Validate(); len(errs) > 0 {
		writeJSON(w, 400, map[string]any{"errors": errs})
		return
	}
	if c.FuncomToken == "" {
		writeJSON(w, 400, map[string]any{"errors": []string{"Funcom token is required"}})
		return
	}
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		writeJSON(w, 409, map[string]any{"error": "an installation is already running"})
		return
	}
	s.running = true
	s.events = nil
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.mu.Unlock()
	writeJSON(w, 202, map[string]any{"started": true})

	go func() {
		defer func() {
			s.mu.Lock()
			s.running = false
			s.cancel = nil
			s.mu.Unlock()
			cancel()
		}()
		run, err := s.connect(ctx, c)
		if err != nil {
			s.emit(install.Event{Type: "error", Msg: sanitize(err.Error(), c)})
			return
		}
		out, _, err := transport.Capture(ctx, run, install.ProbeScript(c))
		if err != nil {
			s.emit(install.Event{Type: "error", Msg: sanitize(err.Error(), c)})
			run.Close()
			return
		}
		info := probe.Parse(out)
		rep := sizing.Analyse(info, c)
		if !rep.OK {
			for _, f := range rep.Findings {
				if f.Level == sizing.Error {
					s.emit(install.Event{Type: "error", Msg: f.Message})
				}
			}
			run.Close()
			return
		}
		o := install.Options{
			Cfg: c, Runner: run, Host: info, Report: rep, Emit: s.emit,
			ExperimentalCombat: rq.ExperimentalCombat,
			Confirm:            s.askConfirm,
			Reconnect:          func(ctx context.Context) (transport.Runner, error) { return s.connect(ctx, c) },
		}
		if err = install.Run(ctx, o); err != nil { // Run closes the runner
			s.emit(install.Event{Type: "error", Msg: sanitize(err.Error(), c)})
		}
	}()
}

func (s *Server) askConfirm(ctx context.Context, id string, d assistant.Diagnosis) bool {
	key := id + "-" + randHex()
	ch := make(chan bool, 1)
	s.mu.Lock()
	s.confirm[key] = ch
	s.mu.Unlock()
	s.emit(install.Event{Type: "confirm", ConfirmID: key, Diagnosis: &d})
	select {
	case v := <-ch:
		return v
	case <-ctx.Done():
		return false
	}
}

func randHex() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func (s *Server) handleConfirm(w http.ResponseWriter, r *http.Request) {
	var rq struct {
		ID      string `json:"id"`
		Approve bool   `json:"approve"`
	}
	if err := decode(r, &rq); err != nil {
		writeJSON(w, 400, map[string]any{"error": "invalid request"})
		return
	}
	s.mu.Lock()
	ch, ok := s.confirm[rq.ID]
	delete(s.confirm, rq.ID)
	s.mu.Unlock()
	if !ok {
		writeJSON(w, 404, map[string]any{"error": "unknown confirmation"})
		return
	}
	ch <- rq.Approve
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) handleCancel(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	if s.cancel != nil {
		s.cancel()
	}
	s.mu.Unlock()
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	fl, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", 500)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	wake := make(chan struct{}, 1)
	wake <- struct{}{}
	s.mu.Lock()
	s.subs[wake] = struct{}{}
	s.mu.Unlock()
	defer func() { s.mu.Lock(); delete(s.subs, wake); s.mu.Unlock() }()
	idx := 0
	tk := time.NewTicker(20 * time.Second)
	defer tk.Stop()
	for {
		select {
		case <-wake:
			s.mu.Lock()
			if idx > len(s.events) { // a new run reset the buffer
				idx = 0
			}
			batch := append([]install.Event(nil), s.events[idx:]...)
			idx = len(s.events)
			s.mu.Unlock()
			for _, e := range batch {
				b, _ := json.Marshal(e)
				if _, err := io.WriteString(w, "data: "+string(b)+"\n\n"); err != nil {
					return
				}
			}
			fl.Flush()
		case <-tk.C:
			_, _ = io.WriteString(w, ": keepalive\n\n")
			fl.Flush()
		case <-r.Context().Done():
			return
		}
	}
}
