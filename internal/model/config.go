// Package model defines the installation plan the UI collects and the
// installer executes. It is pure data plus validation: no I/O.
package model

import (
	"fmt"
	"net"
	"regexp"
	"slices"
	"strings"
)

// TargetKind says where Dune Docker is installed.
type TargetKind string

const (
	TargetLocal TargetKind = "local" // the machine running this installer
	TargetSSH   TargetKind = "ssh"   // a remote Linux server
)

// Mode is the combat ruleset of a Sietch or Deep Desert instance.
type Mode string

const (
	PvE Mode = "PVE"
	PvP Mode = "PVP"
)

// AuthMethod for SSH.
type AuthMethod string

const (
	AuthKey      AuthMethod = "key"
	AuthPassword AuthMethod = "password"
	AuthAgent    AuthMethod = "agent"
)

// SSHConfig describes a remote target. Secrets are never serialised.
type SSHConfig struct {
	Host       string     `json:"host"`
	Port       int        `json:"port"`
	User       string     `json:"user"`
	Auth       AuthMethod `json:"auth"`
	KeyPath    string     `json:"keyPath,omitempty"`
	Password   string     `json:"-"` // login password (also used for sudo if needed)
	Passphrase string     `json:"-"` // key passphrase
	// HostKeyFingerprint is the SHA256 fingerprint the user confirmed
	// (trust on first use). Connections fail if the server presents another.
	HostKeyFingerprint string `json:"hostKeyFingerprint,omitempty"`
}

// Sietch is one Hagga Basin instance.
type Sietch struct {
	Name string `json:"name"`
	Mode Mode   `json:"mode"`
}

// DeepDesert mirrors `dune deepdesert layout set <1|2|3> [--third-role]`.
//
// Upstream semantics: the first instance keeps its existing role, the second
// gets the opposite role, and only the third is freely selectable.
type DeepDesert struct {
	Instances int  `json:"instances"` // 0 = leave Deep Desert at the upstream default
	ThirdRole Mode `json:"thirdRole,omitempty"`
}

// Config is the full install plan.
type Config struct {
	Target TargetKind `json:"target"`
	SSH    SSHConfig  `json:"ssh"`
	// WSLDistro selects the Ubuntu distribution for local installs on Windows.
	WSLDistro   string     `json:"wslDistro,omitempty"`
	InstallDir  string     `json:"installDir,omitempty"` // empty = ~/dune-awakening-selfhost-docker
	WebPort     int        `json:"webPort"`
	ServerTitle string     `json:"serverTitle"`
	Region      string     `json:"region"`
	FuncomToken string     `json:"-"` // never stored on the client
	Sietches    []Sietch   `json:"sietches"`
	DeepDesert  DeepDesert `json:"deepDesert"`
	// ConfigureFirewall adds allow rules to an active ufw/firewalld on the
	// target (never enables or resets a firewall).
	ConfigureFirewall bool `json:"configureFirewall"`
	// ConsoleAllowFrom restricts the web console port to this IP/CIDR
	// (empty = open to everyone).
	ConsoleAllowFrom string `json:"consoleAllowFrom,omitempty"`
	// UpstreamLatest installs the newest upstream release instead of the
	// version DD-Installer was tested with (may be incompatible).
	UpstreamLatest bool `json:"upstreamLatest"`
	// ApplyTuning applies the sizing recommendations (swap, balancer, ...).
	ApplyTuning bool `json:"applyTuning"`
}

// Regions accepted by upstream (SERVER_REGION).
var Regions = []string{"Europe", "North America", "South America", "Asia", "Oceania"}

// Upstream rejects "unsupported characters" without documenting the set, so
// the installer is deliberately conservative.
var nameRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9 _-]{0,31}$`)

var distroRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

const MaxSietches = 8 // installer-side sanity cap; real limit is host memory

// Default returns a sensible starting plan.
func Default() Config {
	return Config{
		Target:      TargetLocal,
		SSH:         SSHConfig{Port: 22, Auth: AuthKey},
		WebPort:     8088,
		ServerTitle: "My Dune Server",
		Region:      "Europe",
		Sietches:    []Sietch{{Name: "Sietch 1", Mode: PvE}},
		ApplyTuning: true,
	}
}

// Validate returns every problem found (empty slice = valid).
func (c *Config) Validate() []string {
	var errs []string
	add := func(f string, a ...any) { errs = append(errs, fmt.Sprintf(f, a...)) }

	switch c.Target {
	case TargetLocal:
	case TargetSSH:
		if strings.TrimSpace(c.SSH.Host) == "" {
			add("ssh: host is required")
		}
		if c.SSH.Port < 1 || c.SSH.Port > 65535 {
			add("ssh: port must be 1-65535")
		}
		if strings.TrimSpace(c.SSH.User) == "" {
			add("ssh: user is required")
		}
		if c.SSH.User == "root" {
			add("ssh: Dune Docker refuses to install as root; use a regular user with sudo")
		}
		switch c.SSH.Auth {
		case AuthKey:
			if c.SSH.KeyPath == "" {
				add("ssh: key path is required for key authentication")
			}
		case AuthPassword:
			if c.SSH.Password == "" {
				add("ssh: password is required")
			}
		case AuthAgent:
		default:
			add("ssh: unknown auth method %q", c.SSH.Auth)
		}
	default:
		add("target must be %q or %q", TargetLocal, TargetSSH)
	}

	if f := strings.TrimSpace(c.ConsoleAllowFrom); f != "" {
		if _, _, err := net.ParseCIDR(f); err != nil && net.ParseIP(f) == nil {
			add("consoleAllowFrom must be an IP address or CIDR such as 203.0.113.7 or 203.0.113.0/24")
		}
	}
	if c.WSLDistro != "" && !distroRe.MatchString(c.WSLDistro) {
		add("wslDistro contains invalid characters")
	}
	if c.InstallDir != "" && !strings.HasPrefix(c.InstallDir, "/") && !strings.HasPrefix(c.InstallDir, "~/") {
		add("installDir must be an absolute Linux path (or start with ~/)")
	}
	if strings.ContainsAny(c.InstallDir, "'\"`$\\;&|<>\n") {
		add("installDir contains characters that are not allowed")
	}
	if c.WebPort < 1 || c.WebPort > 65535 {
		add("webPort must be 1-65535")
	}
	if t := strings.TrimSpace(c.ServerTitle); t == "" || len(t) > 64 || strings.ContainsAny(t, "\"\\`$\n\r") {
		add("serverTitle must be 1-64 characters without quotes, backslashes, $ or backticks")
	}
	if !slices.Contains(Regions, c.Region) {
		add("region must be one of %s", strings.Join(Regions, ", "))
	}

	if len(c.Sietches) < 1 {
		add("at least one Sietch is required")
	}
	if len(c.Sietches) > MaxSietches {
		add("at most %d Sietches can be configured by the installer", MaxSietches)
	}
	seen := map[string]bool{}
	for i, s := range c.Sietches {
		if !nameRe.MatchString(s.Name) {
			add("sietch %d: name %q must be 1-32 letters, digits, spaces, '_' or '-'", i+1, s.Name)
		}
		k := strings.ToLower(s.Name)
		if seen[k] {
			add("sietch %d: duplicate name %q", i+1, s.Name)
		}
		seen[k] = true
		if s.Mode != PvE && s.Mode != PvP {
			add("sietch %d: mode must be PVE or PVP", i+1)
		}
	}

	if d := c.DeepDesert; d.Instances != 0 {
		if d.Instances < 1 || d.Instances > 3 {
			add("deepDesert.instances must be 0 (default), 1, 2 or 3")
		}
		if d.Instances == 3 && d.ThirdRole != PvE && d.ThirdRole != PvP {
			add("deepDesert.thirdRole must be PVE or PVP when 3 instances are used")
		}
	}
	return errs
}

// DeepDesertRoles describes the role each instance receives. The first role
// is "existing" because upstream does not let the installer choose it.
func (d DeepDesert) DeepDesertRoles() []string {
	switch d.Instances {
	case 1:
		return []string{"existing role"}
	case 2:
		return []string{"existing role", "opposite of the first"}
	case 3:
		return []string{"existing role", "opposite of the first", string(d.ThirdRole)}
	}
	return nil
}

// CheckToken validates a Funcom self-host token as typed or pasted. The
// format is not documented, so only clear mistakes are rejected (problem);
// suspicious values only produce a warning.
func CheckToken(raw string) (token, problem, warning string) {
	token = strings.TrimSpace(raw)
	switch {
	case token == "":
		return token, "Enter your Funcom token.", ""
	case strings.ContainsAny(token, " \t\r\n"):
		return token, "The token contains spaces or line breaks - paste only the token itself.", ""
	}
	for _, r := range token {
		if r < 0x21 || r > 0x7e {
			return token, "The token contains characters that cannot be part of a token (non-ASCII or control characters).", ""
		}
	}
	if strings.ContainsAny(token, "\"'`$\\") {
		return token, "The token contains quotes or other unusual characters - did you copy extra text?", ""
	}
	if len(token) < 20 {
		warning = "The token is unusually short; double-check that you copied all of it."
	}
	return token, "", warning
}
