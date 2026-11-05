package transport

import (
	"context"
	"encoding/pem"
	"errors"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"

	"github.com/dev-prophet-code/dd-installer/internal/model"
)

func baseCfg(s *testServer) model.SSHConfig {
	return model.SSHConfig{Host: "127.0.0.1", Port: s.Port, User: "dune", Auth: model.AuthPassword, Password: "pw", HostKeyFingerprint: fingerprint(s)}
}

// noKnownHosts makes the tests independent of the real ~/.ssh/known_hosts.
func noKnownHosts(t *testing.T) {
	t.Helper()
	old := knownHostsPath
	knownHostsPath = func() string { return "" }
	t.Cleanup(func() { knownHostsPath = old })
}

func writeKnownHosts(t *testing.T, entries map[string]ssh.PublicKey) {
	t.Helper()
	p := filepath.Join(t.TempDir(), "known_hosts")
	var b strings.Builder
	for addr, k := range entries {
		b.WriteString(knownhosts.Line([]string{knownhosts.Normalize(addr)}, k) + "\n")
	}
	if err := os.WriteFile(p, []byte(b.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	old := knownHostsPath
	knownHostsPath = func() string { return p }
	t.Cleanup(func() { knownHostsPath = old })
}

func run(t *testing.T, r Runner, script string) (string, int) {
	t.Helper()
	out, code, err := Capture(context.Background(), r, script)
	if err != nil {
		t.Fatal(err)
	}
	return lines(out), code
}

func TestSSHRunAndExitCode(t *testing.T) {
	noKnownHosts(t)
	s := startServer(t, nil)
	c, err := DialSSH(context.Background(), baseCfg(s))
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if out, code := run(t, c, "echo hello\nexit 7\n"); out != "hello" || code != 7 {
		t.Fatalf("out=%q code=%d", out, code)
	}
	// a child that reads stdin must not eat the rest of the script (the eval wrapper)
	if out, _ := run(t, c, "cat >/dev/null\necho after\n"); out != "after" {
		t.Fatalf("stdin was consumed by a child: %q", out)
	}
	if !strings.Contains(c.Describe(), "dune@127.0.0.1") {
		t.Fatal(c.Describe())
	}
}

func TestSSHPasswordViaKeyboardInteractive(t *testing.T) {
	noKnownHosts(t)
	s := startServer(t, func(s *testServer) { s.kbdOnly = true })
	c, err := DialSSH(context.Background(), baseCfg(s))
	if err != nil {
		t.Fatalf("servers that only offer keyboard-interactive must work: %v", err)
	}
	c.Close()
}

func TestSSHWrongPassword(t *testing.T) {
	noKnownHosts(t)
	s := startServer(t, nil)
	cfg := baseCfg(s)
	cfg.Password = "nope"
	if _, err := DialSSH(context.Background(), cfg); err == nil || !strings.Contains(err.Error(), "unable to authenticate") {
		t.Fatalf("expected an authentication error, got %v", err)
	}
}

func TestHostKeyPolicy(t *testing.T) {
	noKnownHosts(t)
	s := startServer(t, nil)

	cfg := baseCfg(s)
	cfg.HostKeyFingerprint = ""
	_, err := DialSSH(context.Background(), cfg)
	var hk *HostKeyError
	if !errors.As(err, &hk) || hk.Mismatch || hk.Fingerprint != fingerprint(s) {
		t.Fatalf("an unconfirmed key must ask the user: %v", err)
	}

	cfg.HostKeyFingerprint = "SHA256:wrong"
	_, err = DialSSH(context.Background(), cfg)
	if !errors.As(err, &hk) || !hk.Mismatch {
		t.Fatalf("a wrong pin must be reported as changed key: %v", err)
	}

	fp, err := ScanHostKey(context.Background(), model.SSHConfig{Host: "127.0.0.1", Port: s.Port})
	if err != nil || fp != fingerprint(s) {
		t.Fatalf("scan: %q %v", fp, err)
	}
}

func TestKnownHostsTrustAndConflict(t *testing.T) {
	s := startServer(t, nil)
	cfg := baseCfg(s)
	cfg.HostKeyFingerprint = "" // no pin: only known_hosts can make it acceptable

	writeKnownHosts(t, map[string]ssh.PublicKey{s.Addr: s.hostKey.PublicKey()})
	c, err := DialSSH(context.Background(), cfg)
	if err != nil {
		t.Fatalf("a key present in known_hosts must be trusted without a prompt: %v", err)
	}
	c.Close()

	other, _ := newKey(t)
	writeKnownHosts(t, map[string]ssh.PublicKey{s.Addr: other.PublicKey()})
	_, err = DialSSH(context.Background(), cfg)
	var hk *HostKeyError
	if !errors.As(err, &hk) || !hk.Mismatch || hk.Source != "known_hosts" {
		t.Fatalf("a conflicting known_hosts entry must be an error: %v", err)
	}
	// even with a (wrong) pin the conflict must not be silently accepted
	cfg.HostKeyFingerprint = fingerprint(s)
	if _, err = DialSSH(context.Background(), cfg); err == nil {
		t.Fatal("known_hosts conflict must win over a session pin")
	}
}

func TestJumpHost(t *testing.T) {
	jump := startServer(t, nil)
	target := startServer(t, nil)

	cfg := baseCfg(target)
	cfg.Jump = "jumper@" + jump.Addr

	noKnownHosts(t)
	if _, err := DialSSH(context.Background(), cfg); err == nil || !strings.Contains(err.Error(), "known_hosts") {
		t.Fatalf("an unknown jump host must be refused with an explanation: %v", err)
	}

	writeKnownHosts(t, map[string]ssh.PublicKey{jump.Addr: jump.hostKey.PublicKey()})
	c, err := DialSSH(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if out, _ := run(t, c, "echo through-the-jump\n"); out != "through-the-jump" {
		t.Fatalf("got %q", out)
	}
	if !strings.Contains(c.Describe(), "via") {
		t.Fatal("description should mention the jump")
	}
	if fp, err := ScanHostKey(context.Background(), cfg); err != nil || fp != fingerprint(target) {
		t.Fatalf("scan through the jump: %q %v", fp, err)
	}
}

func TestParseJump(t *testing.T) {
	u, h, p, err := ParseJump("bob@jump.example.com:2200", "x")
	if err != nil || u != "bob" || h != "jump.example.com" || p != 2200 {
		t.Fatal(u, h, p, err)
	}
	u, h, p, err = ParseJump("jump.example.com", "alice")
	if err != nil || u != "alice" || h != "jump.example.com" || p != 22 {
		t.Fatal(u, h, p, err)
	}
	for _, bad := range []string{"", "a@b,c@d", "bob@:22", "bob@h:99999", "@h"} {
		if _, _, _, err := ParseJump(bad, ""); err == nil {
			t.Errorf("%q should be rejected", bad)
		}
	}
}

func TestPrivateKeyAuth(t *testing.T) {
	noKnownHosts(t)
	signer, priv := newKey(t)
	s := startServer(t, func(s *testServer) { s.password = ""; s.allowed = []ssh.PublicKey{signer.PublicKey()} })
	pemBlock, err := ssh.MarshalPrivateKey(priv, "test")
	if err != nil {
		t.Fatal(err)
	}
	keyFile := filepath.Join(t.TempDir(), "id")
	os.WriteFile(keyFile, pemBytes(pemBlock), 0o600)
	cfg := baseCfg(s)
	cfg.Auth, cfg.KeyPath, cfg.Password = model.AuthKey, keyFile, ""
	c, err := DialSSH(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	c.Close()
}

func TestAgentAuth(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a unix socket agent")
	}
	noKnownHosts(t)
	signer, priv := newKey(t)
	s := startServer(t, func(s *testServer) { s.password = ""; s.allowed = []ssh.PublicKey{signer.PublicKey()} })

	kr := agent.NewKeyring()
	if err := kr.Add(agent.AddedKey{PrivateKey: priv}); err != nil {
		t.Fatal(err)
	}
	sock := filepath.Join(t.TempDir(), "agent.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go agent.ServeAgent(kr, c)
		}
	}()
	t.Setenv("SSH_AUTH_SOCK", sock)
	cfg := baseCfg(s)
	cfg.Auth, cfg.Password = model.AuthAgent, ""
	c, err := DialSSH(context.Background(), cfg)
	if err != nil {
		t.Fatalf("agent auth: %v", err)
	}
	c.Close()

	t.Setenv("SSH_AUTH_SOCK", "")
	if _, err := DialSSH(context.Background(), cfg); err == nil || !strings.Contains(err.Error(), "SSH_AUTH_SOCK") {
		t.Fatalf("a missing agent must explain itself: %v", err)
	}
}

func TestDroppedConnectionIsDetected(t *testing.T) {
	noKnownHosts(t)
	s := startServer(t, nil)
	c, err := DialSSH(context.Background(), baseCfg(s))
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	go func() { time.Sleep(300 * time.Millisecond); s.DropConnections() }()
	_, _, err = Capture(context.Background(), c, "sleep 5\n")
	if err == nil || !IsConnectionLost(err) {
		t.Fatalf("a dropped connection must be reported as lost, got %v", err)
	}
}

func TestRunHonoursContextCancel(t *testing.T) {
	noKnownHosts(t)
	s := startServer(t, nil)
	c, err := DialSSH(context.Background(), baseCfg(s))
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, _, err = Capture(ctx, c, "sleep 5\n")
	if err == nil || time.Since(start) > 3*time.Second {
		t.Fatalf("cancel must return promptly with an error (err=%v after %v)", err, time.Since(start))
	}
}

func pemBytes(b *pem.Block) []byte { return pem.EncodeToMemory(b) }
