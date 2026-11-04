package assistant

import "testing"

func ids(ds []Diagnosis) map[string]Diagnosis {
	m := map[string]Diagnosis{}
	for _, d := range ds {
		m[d.RuleID] = d
	}
	return m
}

func TestRules(t *testing.T) {
	cases := []struct {
		step, out, rule string
		sudoPass        bool
	}{
		{"bootstrap", "Installation could not continue: tar is missing. Install curl and tar", "missing-tools", false},
		{"bootstrap", "/x already contains Dune Docker. Run its existing install.sh", "install-dir-exists", false},
		{"install", "permission denied while trying to connect: /var/run/docker.sock", "docker-socket", false},
		{"install", "Port 8088 is already in use.", "port-in-use", false},
		{"install", "sudo: a password is required", "sudo-password", true},
		{"install", "sudo: a password is required", "sudo-password-manual", false},
		{"install", "E: Could not get lock /var/lib/dpkg/lock-frontend", "pkg-lock", false},
		{"bootstrap", "write error: No space left on device", "no-space", false},
		{"console-setup", "DD_ERR console-not-reachable: connection refused", "console-down", false},
		{"console-setup", "DD_ERR token rejected: 400", "funcom-token", false},
		{"console-setup", "Too many sign-in attempts (HTTP 429)", "login-rate-limit", false},
		{"connect", "dial tcp 1.2.3.4:22: connect: connection refused", "ssh-refused", false},
		{"connect", "ssh: unable to authenticate, attempted methods [none]", "ssh-auth", false},
		{"install", "This project must not be installed as root.", "root-install", false},
	}
	for _, c := range cases {
		got := ids(Diagnose(Context{Step: c.step, Output: c.out, User: "dune", Dir: "$HOME/x", SudoPass: c.sudoPass}))
		if _, ok := got[c.rule]; !ok {
			t.Errorf("%q: rule %s not matched (got %v)", c.out, c.rule, got)
		}
	}
}

func TestNoFalsePositive(t *testing.T) {
	if len(Diagnose(Context{Step: "install", Output: "everything went fine"})) != 0 {
		t.Fatal("unexpected diagnosis")
	}
}

func TestNeverDestructive(t *testing.T) {
	for _, r := range Rules() {
		for _, a := range r.Actions(Context{User: "u", Dir: "d", SudoPass: true}) {
			for _, bad := range []string{"rm -rf", "docker system prune", "volume rm", "mkfs", "dd if="} {
				if contains2(a.Script, bad) {
					t.Errorf("rule %s contains destructive command %q", r.ID, bad)
				}
			}
		}
	}
}

func contains2(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
