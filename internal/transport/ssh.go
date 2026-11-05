package transport

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"

	"github.com/dev-prophet-code/dd-installer/internal/model"
)

// HostKeyError is returned when the server key has not been confirmed yet or
// does not match. The UI shows Fingerprint to the user; once accepted it is
// stored in SSHConfig.HostKeyFingerprint and the connection is retried.
type HostKeyError struct {
	Fingerprint string
	Mismatch    bool   // a different key than the confirmed / known one
	Source      string // "known_hosts" when the conflict comes from ~/.ssh/known_hosts
	Jump        bool   // the jump host, not the target
}

func (e *HostKeyError) Error() string {
	who := "SSH host key"
	if e.Jump {
		who = "SSH jump host key"
	}
	switch {
	case e.Mismatch && e.Source == "known_hosts":
		return who + " CHANGED compared to ~/.ssh/known_hosts: " + e.Fingerprint
	case e.Mismatch:
		return who + " changed: expected the confirmed fingerprint, got " + e.Fingerprint
	}
	return who + " not yet confirmed: " + e.Fingerprint
}

// HostKeyRevokedError is returned when the server key is marked @revoked in
// ~/.ssh/known_hosts. Unlike HostKeyError it can never be confirmed in the UI:
// someone deliberately declared this key untrustworthy.
type HostKeyRevokedError struct {
	Fingerprint string
	Jump        bool
}

func (e *HostKeyRevokedError) Error() string {
	who := "The SSH host key"
	if e.Jump {
		who = "The SSH jump host key"
	}
	return who + " (" + e.Fingerprint + ") is marked @revoked in ~/.ssh/known_hosts. DD-Installer refuses to connect. " +
		"Remove the @revoked line only if you are sure the key is trustworthy again."
}

// SSH is a Runner over an SSH connection.
type SSH struct {
	cfg     model.SSHConfig
	client  *ssh.Client
	jump    *ssh.Client // set when the connection goes through a jump host
	closers []io.Closer
}

// knownHostsPath is overridable for tests.
var knownHostsPath = func() string {
	h, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(h, ".ssh", "known_hosts")
}

// loadKnownHosts returns a callback for ~/.ssh/known_hosts, or nil when the
// file is missing or unreadable (the user is then asked to confirm instead).
func loadKnownHosts() ssh.HostKeyCallback {
	p := knownHostsPath()
	if p == "" {
		return nil
	}
	if _, err := os.Stat(p); err != nil {
		return nil
	}
	cb, err := knownhosts.New(p)
	if err != nil {
		return nil
	}
	return cb
}

// policy decides whether a host key is acceptable.
//
//  1. a matching entry in ~/.ssh/known_hosts is trusted;
//  2. a conflicting entry there is an error (possible interception);
//  3. otherwise the fingerprint the user confirmed in this session must match
//     (trust on first use, never silent). requireKnown (jump hosts) forbids 3.
type policy struct {
	pinned       string
	known        ssh.HostKeyCallback
	requireKnown bool
	jump         bool
	// err is exactly what check returned. connect() reports it instead of
	// guessing from the handshake error, so that a wrong password on a trusted
	// host is not mistaken for a host-key problem.
	err error
}

func (p *policy) check(hostname string, remote net.Addr, key ssh.PublicKey) error {
	p.err = p.decide(hostname, remote, key)
	return p.err
}

func (p *policy) decide(hostname string, remote net.Addr, key ssh.PublicKey) error {
	fp := ssh.FingerprintSHA256(key)
	if p.known != nil {
		err := p.known(hostname, remote, key)
		if err == nil {
			return nil
		}
		// A revoked key is final: neither a session pin nor a confirmation may override it.
		var re *knownhosts.RevokedError
		if errors.As(err, &re) {
			return &HostKeyRevokedError{Fingerprint: fp, Jump: p.jump}
		}
		var ke *knownhosts.KeyError
		if errors.As(err, &ke) && len(ke.Want) > 0 {
			return &HostKeyError{Fingerprint: fp, Mismatch: true, Source: "known_hosts", Jump: p.jump}
		}
	}
	if p.requireKnown {
		return fmt.Errorf("the jump host is not in ~/.ssh/known_hosts. Connect to it once with `ssh` in a terminal to trust it (fingerprint %s)", fp)
	}
	if p.pinned == "" {
		return &HostKeyError{Fingerprint: fp, Jump: p.jump}
	}
	if fp != p.pinned {
		return &HostKeyError{Fingerprint: fp, Mismatch: true, Jump: p.jump}
	}
	return nil
}

func hostAddr(host string, port int) string {
	if port == 0 {
		port = 22
	}
	return net.JoinHostPort(host, strconv.Itoa(port))
}

// ParseJump splits "[user@]host[:port]" (OpenSSH ProxyJump syntax, one hop).
func ParseJump(j string, defUser string) (user, host string, port int, err error) {
	j = strings.TrimSpace(j)
	if j == "" || strings.Contains(j, ",") {
		return "", "", 0, errors.New("only a single jump host is supported (user@host[:port])")
	}
	user = defUser
	if i := strings.LastIndex(j, "@"); i >= 0 {
		user, j = j[:i], j[i+1:]
	}
	port = 22
	if h, p, e := net.SplitHostPort(j); e == nil {
		n, e2 := strconv.Atoi(p)
		if e2 != nil || n < 1 || n > 65535 {
			return "", "", 0, errors.New("invalid jump host port")
		}
		host, port = h, n
	} else {
		host = strings.Trim(j, "[]")
	}
	if host == "" || user == "" {
		return "", "", 0, errors.New("the jump host needs a host and a user (user@host[:port])")
	}
	return user, host, port, nil
}

// connect opens an SSH client connection to cfg, going through cfg.Jump when set.
func connect(ctx context.Context, cfg model.SSHConfig, pol *policy) (client, jump *ssh.Client, err error) {
	auth, closeAgent, err := authMethods(cfg)
	if err != nil {
		return nil, nil, err
	}
	if closeAgent != nil {
		defer closeAgent()
	}
	addr := hostAddr(cfg.Host, cfg.Port)
	var conn net.Conn
	if cfg.Jump != "" {
		ju, jh, jp, perr := ParseJump(cfg.Jump, cfg.User)
		if perr != nil {
			return nil, nil, perr
		}
		jcfg := cfg
		jcfg.Jump, jcfg.User, jcfg.Host, jcfg.Port, jcfg.HostKeyFingerprint = "", ju, jh, jp, ""
		jpol := &policy{known: loadKnownHosts(), requireKnown: true, jump: true}
		jclient, _, jerr := connect(ctx, jcfg, jpol)
		if jerr != nil {
			return nil, nil, fmt.Errorf("jump host %s: %w", hostAddr(jh, jp), jerr)
		}
		conn, err = jclient.Dial("tcp", addr)
		if err != nil {
			jclient.Close()
			return nil, nil, fmt.Errorf("jump host cannot reach %s: %w", addr, err)
		}
		jump = jclient
	} else {
		d := &net.Dialer{Timeout: 15 * time.Second}
		conn, err = d.DialContext(ctx, "tcp", addr)
		if err != nil {
			return nil, nil, err
		}
	}
	cc := &ssh.ClientConfig{User: cfg.User, Auth: auth, HostKeyCallback: pol.check, Timeout: 15 * time.Second}
	sc, chans, reqs, err := ssh.NewClientConn(conn, addr, cc)
	if err != nil {
		conn.Close()
		if jump != nil {
			jump.Close()
		}
		// Only a failed host-key decision is reported as such. Any other
		// failure (wrong password, no matching key, ...) keeps its own message.
		var hk *HostKeyError
		var rv *HostKeyRevokedError
		if errors.As(pol.err, &hk) || errors.As(pol.err, &rv) {
			return nil, nil, pol.err
		}
		if pol.err != nil { // e.g. the jump-host rule: not in known_hosts
			return nil, nil, pol.err
		}
		return nil, nil, fmt.Errorf("ssh connect %s: %w", addr, err)
	}
	return ssh.NewClient(sc, chans, reqs), jump, nil
}

// ScanHostKey connects only to read the server's host key fingerprint (through
// the jump host if configured). It never authenticates.
func ScanHostKey(ctx context.Context, cfg model.SSHConfig) (string, error) {
	var jump *ssh.Client
	var conn net.Conn
	var err error
	addr := hostAddr(cfg.Host, cfg.Port)
	if cfg.Jump != "" {
		ju, jh, jp, perr := ParseJump(cfg.Jump, cfg.User)
		if perr != nil {
			return "", perr
		}
		jcfg := cfg
		jcfg.Jump, jcfg.User, jcfg.Host, jcfg.Port = "", ju, jh, jp
		jclient, _, jerr := connect(ctx, jcfg, &policy{known: loadKnownHosts(), requireKnown: true, jump: true})
		if jerr != nil {
			return "", fmt.Errorf("jump host: %w", jerr)
		}
		jump = jclient
		defer jump.Close()
		conn, err = jump.Dial("tcp", addr)
	} else {
		conn, err = (&net.Dialer{Timeout: 10 * time.Second}).DialContext(ctx, "tcp", addr)
	}
	if err != nil {
		return "", err
	}
	defer conn.Close()
	var fp string
	revoked := false
	known := loadKnownHosts()
	sc, _, _, err := ssh.NewClientConn(conn, addr, &ssh.ClientConfig{
		User: "dd-installer-probe",
		HostKeyCallback: func(h string, a net.Addr, k ssh.PublicKey) error {
			fp = ssh.FingerprintSHA256(k)
			if known != nil {
				var re *knownhosts.RevokedError
				if kerr := known(h, a, k); kerr != nil && errors.As(kerr, &re) {
					revoked = true
				}
			}
			return errors.New("fingerprint captured")
		},
		Timeout: 10 * time.Second,
	})
	if sc != nil {
		sc.Close()
	}
	if revoked {
		return "", &HostKeyRevokedError{Fingerprint: fp}
	}
	if fp != "" {
		return fp, nil
	}
	return "", err
}

// DialSSH opens a connection using cfg. The host key must be known
// (~/.ssh/known_hosts) or match cfg.HostKeyFingerprint; otherwise a
// *HostKeyError is returned so the caller can ask the user to confirm it.
func DialSSH(ctx context.Context, cfg model.SSHConfig) (*SSH, error) {
	pol := &policy{pinned: cfg.HostKeyFingerprint, known: loadKnownHosts()}
	client, jump, err := connect(ctx, cfg, pol)
	if err != nil {
		return nil, err
	}
	return &SSH{cfg: cfg, client: client, jump: jump}, nil
}

func authMethods(cfg model.SSHConfig) ([]ssh.AuthMethod, func(), error) {
	switch cfg.Auth {
	case model.AuthPassword:
		pw := cfg.Password
		return []ssh.AuthMethod{
			ssh.Password(pw),
			// many servers only offer PAM "keyboard-interactive" for passwords
			ssh.KeyboardInteractive(func(_, _ string, questions []string, _ []bool) ([]string, error) {
				answers := make([]string, len(questions))
				for i := range answers {
					answers[i] = pw
				}
				return answers, nil
			}),
		}, nil, nil
	case model.AuthAgent:
		rw, err := dialAgent()
		if err != nil {
			return nil, nil, err
		}
		return []ssh.AuthMethod{ssh.PublicKeysCallback(agent.NewClient(rw).Signers)}, func() { rw.Close() }, nil
	case model.AuthKey:
		pem, err := os.ReadFile(expandHome(cfg.KeyPath))
		if err != nil {
			return nil, nil, fmt.Errorf("read key: %w", err)
		}
		var signer ssh.Signer
		if cfg.Passphrase != "" {
			signer, err = ssh.ParsePrivateKeyWithPassphrase(pem, []byte(cfg.Passphrase))
		} else {
			signer, err = ssh.ParsePrivateKey(pem)
		}
		if err != nil {
			var pm *ssh.PassphraseMissingError
			if errors.As(err, &pm) {
				return nil, nil, errors.New("the private key is passphrase-protected; enter the passphrase")
			}
			return nil, nil, fmt.Errorf("parse key: %w", err)
		}
		return []ssh.AuthMethod{ssh.PublicKeys(signer)}, nil, nil
	}
	return nil, nil, fmt.Errorf("unknown auth method %q", cfg.Auth)
}

func expandHome(p string) string {
	if strings.HasPrefix(p, "~/") || strings.HasPrefix(p, `~\`) {
		if h, err := os.UserHomeDir(); err == nil {
			return filepath.Join(h, p[2:])
		}
	}
	return p
}

func (s *SSH) Describe() string {
	d := fmt.Sprintf("%s@%s:%d", s.cfg.User, s.cfg.Host, s.cfg.Port)
	if s.cfg.Jump != "" {
		d += " via " + s.cfg.Jump
	}
	return d
}

func (s *SSH) Close() error {
	err := s.client.Close()
	if s.jump != nil {
		_ = s.jump.Close()
	}
	return err
}

func (s *SSH) Run(ctx context.Context, script string, out io.Writer) (int, error) {
	sess, err := s.client.NewSession()
	if err != nil {
		return -1, err
	}
	defer sess.Close()
	sess.Stdin = strings.NewReader(script)
	sess.Stdout = out
	sess.Stderr = out

	done := make(chan error, 1)
	go func() { done <- sess.Run(ShellCommand) }()
	select {
	case <-ctx.Done():
		sess.Close()
		return -1, ctx.Err()
	case err := <-done:
		var ee *ssh.ExitError
		if errors.As(err, &ee) {
			return ee.ExitStatus(), nil
		}
		if err != nil {
			return -1, err
		}
		return 0, nil
	}
}
