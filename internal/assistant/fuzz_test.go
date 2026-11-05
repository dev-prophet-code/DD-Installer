package assistant

import (
	"strings"
	"testing"
)

// Diagnosing arbitrary output must not panic, and no fix may ever be destructive.
func FuzzDiagnose(f *testing.F) {
	f.Add("install", "permission denied while trying to connect to /var/run/docker.sock", 1)
	f.Add("bootstrap", "tar is missing. no space left on device", 1)
	f.Add("console-setup", "DD_ERR console-not-reachable connection refused too many sign-in attempts", 20)
	f.Fuzz(func(t *testing.T, step, out string, code int) {
		for _, d := range Diagnose(Context{Step: step, Output: out, ExitCode: code, User: "u", Dir: "$HOME/x", SudoPass: true}) {
			for _, a := range d.Actions {
				for _, bad := range []string{"rm -rf", "docker system prune", "volume rm", "mkfs", "dd if="} {
					if strings.Contains(a.Script, bad) {
						t.Fatalf("rule %s produced destructive script", d.RuleID)
					}
				}
			}
		}
	})
}
