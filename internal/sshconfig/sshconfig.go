// Package sshconfig reads the few settings of ~/.ssh/config that the installer
// can use (Host, HostName, User, Port, IdentityFile, ProxyJump). It follows
// OpenSSH's rule "the first value obtained for a keyword wins". Include and
// Match blocks are ignored.
package sshconfig

import (
	"bufio"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// Entry is what applies to one host alias.
type Entry struct {
	Alias        string
	HostName     string
	User         string
	Port         int
	IdentityFile string
	ProxyJump    string
}

type block struct {
	patterns []string
	kv       map[string]string // lower-case keyword -> first value
	order    []string
}

// Config is a parsed file.
type Config struct{ blocks []block }

// Load parses ~/.ssh/config; a missing file yields an empty Config.
func Load() (*Config, error) {
	h, err := os.UserHomeDir()
	if err != nil {
		return &Config{}, nil
	}
	f, err := os.Open(filepath.Join(h, ".ssh", "config"))
	if err != nil {
		return &Config{}, nil
	}
	defer f.Close()
	return Parse(f), nil
}

// Parse reads configuration text.
func Parse(r io.Reader) *Config {
	c := &Config{}
	var cur *block
	ignoring := false // inside an unsupported Match block
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 1<<20)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val := split(line)
		switch strings.ToLower(key) {
		case "host":
			c.blocks = append(c.blocks, block{patterns: strings.Fields(val), kv: map[string]string{}})
			cur = &c.blocks[len(c.blocks)-1]
			ignoring = false
		case "match":
			cur, ignoring = nil, true // unsupported: ignore its settings
		default:
			if ignoring {
				continue
			}
			if cur == nil {
				// settings before any Host apply to all hosts
				c.blocks = append(c.blocks, block{patterns: []string{"*"}, kv: map[string]string{}})
				cur = &c.blocks[len(c.blocks)-1]
			}
			k := strings.ToLower(key)
			if _, seen := cur.kv[k]; !seen {
				cur.kv[k] = unquote(val)
			}
		}
	}
	return c
}

func split(line string) (key, val string) {
	i := strings.IndexAny(line, " \t=")
	if i < 0 {
		return line, ""
	}
	key = line[:i]
	val = strings.TrimLeft(line[i:], " \t=")
	return key, strings.TrimSpace(val)
}

func unquote(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}

// match reports whether pattern list matches alias (with ! negation).
func match(patterns []string, alias string) bool {
	matched := false
	for _, p := range patterns {
		neg := strings.HasPrefix(p, "!")
		p = strings.TrimPrefix(p, "!")
		if globMatch(p, alias) {
			if neg {
				return false
			}
			matched = true
		}
	}
	return matched
}

func globMatch(pattern, s string) bool {
	re := "^" + strings.NewReplacer(`\*`, ".*", `\?`, ".").Replace(regexp.QuoteMeta(pattern)) + "$"
	ok, _ := regexp.MatchString("(?i)"+re, s)
	return ok
}

// Lookup returns the settings for alias. Found is false when no Host block
// names the alias explicitly (only wildcard blocks would apply).
func (c *Config) Lookup(alias string) (e Entry, found bool) {
	e.Alias = alias
	for _, b := range c.blocks {
		if !match(b.patterns, alias) {
			continue
		}
		for _, p := range b.patterns {
			if !strings.ContainsAny(p, "*?!") && strings.EqualFold(p, alias) {
				found = true
			}
		}
		for _, k := range []string{"hostname", "user", "port", "identityfile", "proxyjump"} {
			v, ok := b.kv[k]
			if !ok {
				continue
			}
			switch k {
			case "hostname":
				if e.HostName == "" {
					e.HostName = v
				}
			case "user":
				if e.User == "" {
					e.User = v
				}
			case "port":
				if e.Port == 0 {
					if n, err := strconv.Atoi(v); err == nil && n > 0 && n < 65536 {
						e.Port = n
					}
				}
			case "identityfile":
				if e.IdentityFile == "" {
					e.IdentityFile = v
				}
			case "proxyjump":
				if e.ProxyJump == "" && !strings.EqualFold(v, "none") {
					e.ProxyJump = v
				}
			}
		}
	}
	if e.HostName == "" {
		e.HostName = alias
	}
	return e, found
}

// Aliases lists explicit (non-wildcard) host names, for suggestions.
func (c *Config) Aliases() []string {
	var out []string
	seen := map[string]bool{}
	for _, b := range c.blocks {
		for _, p := range b.patterns {
			if strings.ContainsAny(p, "*?!") || seen[p] {
				continue
			}
			seen[p] = true
			out = append(out, p)
		}
	}
	return out
}
