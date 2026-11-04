package transport

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	"github.com/dev-prophet-code/dd-installer/internal/model"
)

// HostKeyError is returned when the server key has not been confirmed yet.
// The GUI shows Fingerprint to the user; once accepted it is stored in
// SSHConfig.HostKeyFingerprint and the connection is retried.
type HostKeyError struct {
	Fingerprint string
	Mismatch    bool // true: a different key than the confirmed one
}

func (e *HostKeyError) Error() string {
	if e.Mismatch {
		return "SSH host key changed: expected the confirmed fingerprint, got " + e.Fingerprint
	}
	return "SSH host key not yet confirmed: " + e.Fingerprint
}

// SSH is a Runner over an SSH connection.
type SSH struct {
	cfg    model.SSHConfig
	client *ssh.Client
}

// ScanHostKey connects only to read the server's host key fingerprint.
func ScanHostKey(ctx context.Context, host string, port int) (string, error) {
	var fp string
	d := &net.Dialer{Timeout: 10 * time.Second}
	conn, err := d.DialContext(ctx, "tcp", net.JoinHostPort(host, strconv.Itoa(port)))
	if err != nil {
		return "", err
	}
	defer conn.Close()
	sc, _, _, err := ssh.NewClientConn(conn, host, &ssh.ClientConfig{
		User: "dd-installer-probe",
		HostKeyCallback: func(_ string, _ net.Addr, k ssh.PublicKey) error {
			fp = ssh.FingerprintSHA256(k)
			return errors.New("fingerprint captured")
		},
		Timeout: 10 * time.Second,
	})
	if sc != nil {
		sc.Close()
	}
	if fp != "" {
		return fp, nil
	}
	return "", err
}

// DialSSH opens a connection using cfg. The host key must match
// cfg.HostKeyFingerprint; if empty a *HostKeyError is returned so the caller
// can ask the user to confirm it (trust on first use, never silent).
func DialSSH(ctx context.Context, cfg model.SSHConfig) (*SSH, error) {
	auth, closeAgent, err := authMethods(cfg)
	if err != nil {
		return nil, err
	}
	if closeAgent != nil {
		defer closeAgent()
	}
	var seen string
	cc := &ssh.ClientConfig{
		User: cfg.User,
		Auth: auth,
		HostKeyCallback: func(_ string, _ net.Addr, k ssh.PublicKey) error {
			seen = ssh.FingerprintSHA256(k)
			if cfg.HostKeyFingerprint == "" {
				return &HostKeyError{Fingerprint: seen}
			}
			if seen != cfg.HostKeyFingerprint {
				return &HostKeyError{Fingerprint: seen, Mismatch: true}
			}
			return nil
		},
		Timeout: 15 * time.Second,
	}
	addr := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
	d := &net.Dialer{Timeout: 15 * time.Second}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}
	sc, chans, reqs, err := ssh.NewClientConn(conn, addr, cc)
	if err != nil {
		conn.Close()
		var hk *HostKeyError
		if errors.As(err, &hk) {
			return nil, hk
		}
		if seen != "" && cfg.HostKeyFingerprint == "" {
			return nil, &HostKeyError{Fingerprint: seen}
		}
		return nil, fmt.Errorf("ssh connect %s: %w", addr, err)
	}
	return &SSH{cfg: cfg, client: ssh.NewClient(sc, chans, reqs)}, nil
}

func authMethods(cfg model.SSHConfig) ([]ssh.AuthMethod, func(), error) {
	switch cfg.Auth {
	case model.AuthPassword:
		return []ssh.AuthMethod{ssh.Password(cfg.Password)}, nil, nil
	case model.AuthAgent:
		sock := os.Getenv("SSH_AUTH_SOCK")
		if sock == "" {
			return nil, nil, errors.New("SSH_AUTH_SOCK is not set; no ssh-agent available")
		}
		c, err := net.Dial("unix", sock)
		if err != nil {
			return nil, nil, fmt.Errorf("ssh-agent: %w", err)
		}
		return []ssh.AuthMethod{ssh.PublicKeysCallback(agent.NewClient(c).Signers)}, func() { c.Close() }, nil
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
	if strings.HasPrefix(p, "~/") {
		if h, err := os.UserHomeDir(); err == nil {
			return h + p[1:]
		}
	}
	return p
}

func (s *SSH) Describe() string {
	return fmt.Sprintf("%s@%s:%d", s.cfg.User, s.cfg.Host, s.cfg.Port)
}
func (s *SSH) Close() error { return s.client.Close() }

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
