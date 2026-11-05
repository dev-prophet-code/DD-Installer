package transport

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// A wrong password on a host that is already trusted through known_hosts must
// be reported as a login failure, not as a host-key question.
func TestWrongPasswordOnKnownHostIsALoginError(t *testing.T) {
	s := startServer(t, nil)
	cfg := baseCfg(s)
	cfg.HostKeyFingerprint = ""
	cfg.Password = "wrong"
	writeKnownHosts(t, map[string]ssh.PublicKey{s.Addr: s.hostKey.PublicKey()})
	_, err := DialSSH(context.Background(), cfg)
	var hk *HostKeyError
	if err == nil || errors.As(err, &hk) || !strings.Contains(err.Error(), "unable to authenticate") {
		t.Fatalf("expected a plain authentication error, got %T: %v", err, err)
	}
}

func writeRevokedKnownHosts(t *testing.T, addr string, key ssh.PublicKey) {
	t.Helper()
	line := "@revoked " + knownhosts.Line([]string{knownhosts.Normalize(addr)}, key) + "\n"
	p := filepath.Join(t.TempDir(), "known_hosts")
	if err := os.WriteFile(p, []byte(line), 0o600); err != nil {
		t.Fatal(err)
	}
	old := knownHostsPath
	knownHostsPath = func() string { return p }
	t.Cleanup(func() { knownHostsPath = old })
}

// A key marked @revoked is refused, with or without a confirmed fingerprint,
// and it is never offered for confirmation in the UI (no HostKeyError).
func TestRevokedKeyIsAlwaysRefused(t *testing.T) {
	s := startServer(t, nil)
	writeRevokedKnownHosts(t, s.Addr, s.hostKey.PublicKey())

	for _, pin := range []string{"", fingerprint(s)} {
		cfg := baseCfg(s)
		cfg.HostKeyFingerprint = pin
		c, err := DialSSH(context.Background(), cfg)
		if err == nil {
			c.Close()
			t.Fatalf("connected to a revoked key (pin %q)", pin)
		}
		var rv *HostKeyRevokedError
		var hk *HostKeyError
		if !errors.As(err, &rv) || errors.As(err, &hk) {
			t.Fatalf("pin %q: want HostKeyRevokedError only, got %T: %v", pin, err, err)
		}
	}
	// the fingerprint scan (first step of the confirmation dialog) refuses too
	_, err := ScanHostKey(context.Background(), baseCfg(s))
	var rv *HostKeyRevokedError
	if !errors.As(err, &rv) {
		t.Fatalf("ScanHostKey must refuse a revoked key, got %v", err)
	}
}
