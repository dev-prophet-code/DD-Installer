package ui

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2/test"

	"github.com/dev-prophet-code/dd-installer/internal/model"
	"github.com/dev-prophet-code/dd-installer/internal/platform"
)

func sshCfg() model.Config {
	c := model.Default()
	c.Target = model.TargetSSH
	c.SSH = model.SSHConfig{Host: "203.0.113.10", Port: 22, User: "dune", Auth: model.AuthAgent}
	c.WebPort = 8088
	return c
}

func TestConsoleWarningTextFitsTheSituation(t *testing.T) {
	// open to everybody through the firewall helper
	c := sshCfg()
	c.ConfigureFirewall = true
	intro, exp, adv := consoleWarningText(c)
	if !strings.Contains(intro, "plain HTTP") || !strings.Contains(exp, "EVERYONE") || !strings.Contains(exp, "8088") {
		t.Fatalf("open console: %q / %q", intro, exp)
	}
	if !strings.Contains(strings.Join(adv, "\n"), "ssh -L 8088:127.0.0.1:8088 dune@203.0.113.10") {
		t.Fatalf("the SSH tunnel advice is missing or wrong: %v", adv)
	}
	// no firewall option: Docker bypasses ufw
	c.ConfigureFirewall = false
	if _, exp, _ = consoleWarningText(c); !strings.Contains(exp, "Docker publishes") {
		t.Fatalf("no firewall option: %q", exp)
	}
	// restricted
	c.ConsoleAllowFrom = "203.0.113.0/24"
	if _, exp, _ = consoleWarningText(c); !strings.Contains(exp, "203.0.113.0/24") || strings.Contains(exp, "EVERYONE") {
		t.Fatalf("restricted console: %q", exp)
	}
	// local install: no SSH advice
	l := model.Default()
	_, exp, adv = consoleWarningText(l)
	if !strings.Contains(exp, "localhost") || strings.Contains(strings.Join(adv, "\n"), "ssh -L") {
		t.Fatalf("local: %q %v", exp, adv)
	}
}

// The installation only starts after the box was ticked AND Continue was pressed.
func TestInstallNeedsTheAcknowledgement(t *testing.T) {
	started, cancelled := 0, 0
	n := newConsoleNotice(sshCfg(), func() { started++ }, func() { cancelled++ })
	if !n.cont.Disabled() {
		t.Fatal("Continue must be disabled until the notice is acknowledged")
	}
	test.Tap(n.cont) // a tap on a disabled button does nothing
	if started != 0 {
		t.Fatal("the installation started without acknowledgement")
	}
	test.Tap(n.ack)
	if n.cont.Disabled() {
		t.Fatal("Continue must be enabled after ticking the box")
	}
	test.Tap(n.ack) // un-tick: locked again
	if !n.cont.Disabled() {
		t.Fatal("un-ticking must lock Continue again")
	}
	test.Tap(n.ack)
	test.Tap(n.cont)
	if started != 1 || cancelled != 0 {
		t.Fatalf("started=%d cancelled=%d", started, cancelled)
	}
	test.Tap(n.cancel)
	if started != 1 || cancelled != 1 {
		t.Fatalf("Cancel must not start anything: started=%d cancelled=%d", started, cancelled)
	}
}

// Pressing "Install now" shows the notice instead of starting right away.
func TestInstallButtonShowsTheNoticeFirst(t *testing.T) {
	z := newTest(t, &fakePF{id: "linux", ls: platform.LocalSupport{Supported: true}})
	z.onInstall()
	if z.w.Canvas().Overlays().Top() == nil {
		t.Fatal("the notice must be on screen")
	}
	if z.c.Running() {
		t.Fatal("nothing may start before the notice is acknowledged")
	}
}
