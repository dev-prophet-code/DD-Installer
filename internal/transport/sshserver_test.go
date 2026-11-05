package transport

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os/exec"
	"strings"
	"sync"
	"testing"

	"golang.org/x/crypto/ssh"
)

// testServer is a real in-process SSH server: it authenticates (password,
// keyboard-interactive, public key), runs exec requests in a real shell, and
// supports direct-tcpip forwarding (used for jump hosts).
type testServer struct {
	t        *testing.T
	ln       net.Listener
	hostKey  ssh.Signer
	Addr     string
	Port     int
	password string // "" disables password auth
	kbdOnly  bool   // password only via keyboard-interactive
	allowed  []ssh.PublicKey

	mu    sync.Mutex
	conns []net.Conn
}

func newKey(t *testing.T) (ssh.Signer, ed25519.PrivateKey) {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	s, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	return s, priv
}

func startServer(t *testing.T, mut func(*testServer)) *testServer {
	t.Helper()
	hk, _ := newKey(t)
	s := &testServer{t: t, hostKey: hk, password: "pw"}
	if mut != nil {
		mut(s)
	}
	cfg := &ssh.ServerConfig{}
	cfg.AddHostKey(hk)
	if s.password != "" && !s.kbdOnly {
		cfg.PasswordCallback = func(_ ssh.ConnMetadata, p []byte) (*ssh.Permissions, error) {
			if string(p) == s.password {
				return nil, nil
			}
			return nil, fmt.Errorf("bad password")
		}
	}
	if s.password != "" && s.kbdOnly {
		cfg.KeyboardInteractiveCallback = func(_ ssh.ConnMetadata, ch ssh.KeyboardInteractiveChallenge) (*ssh.Permissions, error) {
			ans, err := ch("", "", []string{"Password: "}, []bool{false})
			if err != nil || len(ans) != 1 || ans[0] != s.password {
				return nil, fmt.Errorf("bad password")
			}
			return nil, nil
		}
	}
	if len(s.allowed) > 0 {
		cfg.PublicKeyCallback = func(_ ssh.ConnMetadata, k ssh.PublicKey) (*ssh.Permissions, error) {
			for _, a := range s.allowed {
				if string(a.Marshal()) == string(k.Marshal()) {
					return nil, nil
				}
			}
			return nil, fmt.Errorf("unknown key")
		}
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s.ln = ln
	s.Addr = ln.Addr().String()
	s.Port = ln.Addr().(*net.TCPAddr).Port
	t.Cleanup(s.Close)
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			s.mu.Lock()
			s.conns = append(s.conns, c)
			s.mu.Unlock()
			go s.serve(c, cfg)
		}
	}()
	return s
}

func (s *testServer) Close() {
	s.ln.Close()
	s.DropConnections()
}

// DropConnections abruptly closes every open client connection.
func (s *testServer) DropConnections() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, c := range s.conns {
		c.Close()
	}
	s.conns = nil
}

func (s *testServer) serve(c net.Conn, cfg *ssh.ServerConfig) {
	sc, chans, reqs, err := ssh.NewServerConn(c, cfg)
	if err != nil {
		return
	}
	defer sc.Close()
	go ssh.DiscardRequests(reqs)
	for nc := range chans {
		switch nc.ChannelType() {
		case "session":
			ch, creqs, err := nc.Accept()
			if err != nil {
				continue
			}
			go s.session(ch, creqs)
		case "direct-tcpip":
			var p struct {
				Host     string
				Port     uint32
				OrigHost string
				OrigPort uint32
			}
			if err := ssh.Unmarshal(nc.ExtraData(), &p); err != nil {
				nc.Reject(ssh.ConnectionFailed, "bad payload")
				continue
			}
			dst, err := net.Dial("tcp", net.JoinHostPort(p.Host, fmt.Sprint(p.Port)))
			if err != nil {
				nc.Reject(ssh.ConnectionFailed, err.Error())
				continue
			}
			ch, creqs, _ := nc.Accept()
			go ssh.DiscardRequests(creqs)
			go func() { io.Copy(ch, dst); ch.CloseWrite() }()
			go func() { io.Copy(dst, ch); dst.Close() }()
		default:
			nc.Reject(ssh.UnknownChannelType, "unsupported")
		}
	}
}

func (s *testServer) session(ch ssh.Channel, reqs <-chan *ssh.Request) {
	defer ch.Close()
	for r := range reqs {
		if r.Type != "exec" {
			r.Reply(false, nil)
			continue
		}
		if len(r.Payload) < 4 {
			r.Reply(false, nil)
			continue
		}
		n := binary.BigEndian.Uint32(r.Payload)
		cmdline := string(r.Payload[4 : 4+n])
		r.Reply(true, nil)
		cmd := exec.Command("sh", "-c", cmdline)
		cmd.Stdin = ch
		cmd.Stdout = ch
		cmd.Stderr = ch.Stderr()
		code := 0
		if err := cmd.Run(); err != nil {
			if ee, ok := err.(*exec.ExitError); ok {
				code = ee.ExitCode()
			} else {
				code = 127
			}
		}
		ch.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{uint32(code)}))
		return
	}
}

func fingerprint(s *testServer) string { return ssh.FingerprintSHA256(s.hostKey.PublicKey()) }

func lines(s string) string { return strings.TrimSpace(s) }
