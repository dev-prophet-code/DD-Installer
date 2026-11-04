package gui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const hostPort = "127.0.0.1:4242"

func newTS(t *testing.T) *Server {
	s := New(nil)
	s.SetAddr(hostPort)
	return s
}

func do(s *Server, method, path, host, body string, hdr map[string]string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, "http://"+host+path, strings.NewReader(body))
	r.Host = host
	for k, v := range hdr {
		r.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	return w
}

func TestAuthRequired(t *testing.T) {
	s := newTS(t)
	for _, p := range []string{"/api/env", "/api/events"} {
		if w := do(s, "GET", p, hostPort, "", nil); w.Code != http.StatusUnauthorized {
			t.Errorf("%s without token: %d", p, w.Code)
		}
	}
	if w := do(s, "GET", "/api/env", hostPort, "", map[string]string{"X-DD-Token": s.Token()}); w.Code != 200 {
		t.Errorf("valid token rejected: %d", w.Code)
	}
	if w := do(s, "GET", "/api/env", hostPort, "", map[string]string{"X-DD-Token": s.Token()[:10]}); w.Code != 401 {
		t.Error("partial token accepted")
	}
}

func TestDNSRebindingAndOrigin(t *testing.T) {
	s := newTS(t)
	h := map[string]string{"X-DD-Token": s.Token(), "Content-Type": "application/json"}
	if w := do(s, "GET", "/api/env", "evil.example:4242", "", h); w.Code != 403 {
		t.Errorf("foreign Host accepted: %d", w.Code)
	}
	h["Origin"] = "http://evil.example"
	if w := do(s, "POST", "/api/cancel", hostPort, "{}", h); w.Code != 403 {
		t.Errorf("foreign Origin accepted: %d", w.Code)
	}
	h["Origin"] = "http://" + hostPort
	if w := do(s, "POST", "/api/cancel", hostPort, "{}", h); w.Code != 200 {
		t.Errorf("same origin rejected: %d", w.Code)
	}
}

func TestContentTypeEnforced(t *testing.T) {
	s := newTS(t)
	h := map[string]string{"X-DD-Token": s.Token(), "Content-Type": "text/plain"}
	if w := do(s, "POST", "/api/validate", hostPort, "{}", h); w.Code != http.StatusUnsupportedMediaType {
		t.Errorf("non-JSON POST accepted: %d", w.Code)
	}
}

func TestInstallRequiresDisclaimer(t *testing.T) {
	s := newTS(t)
	h := map[string]string{"X-DD-Token": s.Token(), "Content-Type": "application/json"}
	w := do(s, "POST", "/api/install", hostPort, `{"config":{}}`, h)
	if w.Code != 400 || !strings.Contains(w.Body.String(), "disclaimer") {
		t.Errorf("install without acceptance: %d %s", w.Code, w.Body.String())
	}
}

func TestUnknownFieldsAndOversize(t *testing.T) {
	s := newTS(t)
	h := map[string]string{"X-DD-Token": s.Token(), "Content-Type": "application/json"}
	if w := do(s, "POST", "/api/validate", hostPort, `{"config":{},"evil":1}`, h); w.Code != 400 {
		t.Errorf("unknown field accepted: %d", w.Code)
	}
	big := `{"config":{"serverTitle":"` + strings.Repeat("A", 2<<20) + `"}}`
	if w := do(s, "POST", "/api/validate", hostPort, big, h); w.Code != 400 {
		t.Errorf("oversize body accepted: %d", w.Code)
	}
}

func TestSecurityHeadersAndTokenExchange(t *testing.T) {
	s := newTS(t)
	w := do(s, "GET", "/?t="+s.Token(), hostPort, "", nil)
	if w.Code != http.StatusSeeOther || !strings.Contains(w.Header().Get("Set-Cookie"), "HttpOnly") {
		t.Fatalf("token exchange: %d %v", w.Code, w.Header())
	}
	if !strings.Contains(w.Header().Get("Set-Cookie"), "SameSite=Strict") {
		t.Error("cookie must be SameSite=Strict")
	}
	if w := do(s, "GET", "/?t=wrong", hostPort, "", nil); w.Code != 403 {
		t.Errorf("wrong token: %d", w.Code)
	}
	w = do(s, "GET", "/", hostPort, "", nil)
	if w.Header().Get("Content-Security-Policy") == "" || w.Header().Get("X-Frame-Options") != "DENY" {
		t.Error("security headers missing")
	}
}

func TestValidateNeverEchoesSecrets(t *testing.T) {
	s := newTS(t)
	h := map[string]string{"X-DD-Token": s.Token(), "Content-Type": "application/json"}
	w := do(s, "POST", "/api/validate", hostPort, `{"config":{"target":"ssh"},"secrets":{"sshPassword":"hunter2hunter2","funcomToken":"TOK12345"}}`, h)
	if strings.Contains(w.Body.String(), "hunter2") || strings.Contains(w.Body.String(), "TOK12345") {
		t.Error("secret echoed")
	}
}

func TestPathTraversalStatic(t *testing.T) {
	s := newTS(t)
	for _, p := range []string{"/../server.go", "/..%2fserver.go", "/web/../../go.mod"} {
		w := do(s, "GET", p, hostPort, "", nil)
		if w.Code == 200 && strings.Contains(w.Body.String(), "package gui") {
			t.Errorf("traversal served source via %s", p)
		}
	}
}
