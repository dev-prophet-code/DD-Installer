package windows

import (
	"strings"
	"testing"
)

func TestPickUbuntu(t *testing.T) {
	if got := pickUbuntu([]string{"Debian", "Ubuntu-24.04"}); got != "Ubuntu-24.04" {
		t.Fatal(got)
	}
	if pickUbuntu([]string{"Debian"}) != "" || pickUbuntu(nil) != "" {
		t.Fatal("no ubuntu expected")
	}
}

func TestRebootNeeded(t *testing.T) {
	cases := []struct {
		exit    int
		distros []string
		want    bool
	}{
		{3010, nil, true},
		{0, nil, true}, // feature just enabled, distro appears after reboot
		{0, []string{"Ubuntu"}, false},
		{1, nil, false}, // failure (e.g. UAC declined) is an error, not a reboot
	}
	for _, c := range cases {
		if got := rebootNeeded(c.exit, c.distros); got != c.want {
			t.Errorf("rebootNeeded(%d,%v)=%v", c.exit, c.distros, got)
		}
	}
}

func TestProvisionScript(t *testing.T) {
	s := provisionScript()
	for _, want := range []string{"useradd", "NOPASSWD", "systemd=true", "default=", "visudo -cf"} {
		if !strings.Contains(s, want) {
			t.Errorf("script missing %q", want)
		}
	}
	if strings.Contains(s, "rm -rf") {
		t.Error("destructive command in provisioning script")
	}
}

func TestInstallCommandElevates(t *testing.T) {
	c := strings.Join(installCommand(), " ")
	if !strings.Contains(c, "-Verb RunAs") || !strings.Contains(c, "--no-launch") {
		t.Fatal(c)
	}
}
