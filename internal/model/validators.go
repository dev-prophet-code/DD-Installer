package model

import (
	"errors"
	"fmt"
	"net"
	"strings"
)

// Field validators, shared by Validate and the UI (inline field errors).

// ValidSietchName checks one Sietch name.
func ValidSietchName(n string) error {
	if !nameRe.MatchString(n) {
		return errors.New("1-32 letters, digits, spaces, '_' or '-'")
	}
	return nil
}

// ValidTitle checks the public server title.
func ValidTitle(t string) error {
	t = strings.TrimSpace(t)
	if t == "" || len(t) > 64 || strings.ContainsAny(t, "\"\\`$\n\r") {
		return errors.New("1-64 characters, no quotes, backslashes, $ or backticks")
	}
	return nil
}

// ValidPort checks a TCP port given as text.
func ValidPort(s string) error {
	var n int
	if _, err := fmt.Sscanf(strings.TrimSpace(s), "%d", &n); err != nil || n < 1 || n > 65535 || fmt.Sprint(n) != strings.TrimSpace(s) {
		return errors.New("a number from 1 to 65535")
	}
	return nil
}

// ValidHost checks an SSH host (name, IPv4 or IPv6) or an ~/.ssh/config alias.
func ValidHost(h string) error {
	// The raw value is checked, not a trimmed copy: whitespace or a line break
	// at the end must not slip through into scripts.
	if strings.TrimSpace(h) == "" {
		return errors.New("required")
	}
	if net.ParseIP(strings.Trim(h, "[]")) != nil {
		return nil
	}
	for _, r := range h {
		ok := r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '.' || r == '-' || r == '_'
		if !ok {
			return errors.New("only letters, digits, '.', '-' and '_'")
		}
	}
	return nil
}

// ValidUser checks the SSH user (must not be root).
func ValidUser(u string) error {
	switch {
	case strings.TrimSpace(u) == "":
		return errors.New("required")
	case u == "root":
		return errors.New("use a regular user with sudo, not root")
	}
	for _, r := range u {
		ok := r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '.' || r == '-' || r == '_'
		if !ok {
			return errors.New("only letters, digits, '.', '-' and '_'")
		}
	}
	return nil
}

// ValidAllowFrom checks the optional console restriction (IP or CIDR).
func ValidAllowFrom(f string) error {
	f = strings.TrimSpace(f)
	if f == "" {
		return nil
	}
	if _, _, err := net.ParseCIDR(f); err != nil && net.ParseIP(f) == nil {
		return errors.New("an IP address or CIDR such as 203.0.113.7 or 203.0.113.0/24")
	}
	return nil
}
