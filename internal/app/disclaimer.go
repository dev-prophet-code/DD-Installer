package app

import (
	_ "embed"
	"regexp"
	"strings"
)

// DisclaimerShort is shown permanently in the UI.
const DisclaimerShort = "Unofficial community tool. DD-Installer does NOT store your credentials (SSH password, key passphrase, Funcom token stay in memory for this session only). " +
	"Neither RedBlink (author of Dune: Awakening Docker) nor Prophet (author of DD-Installer) can be held liable for any damage."

// disclaimerMD is the single source of the full disclaimer. docs/DISCLAIMER.md
// is a verbatim copy (kept in sync by a test and scripts/sync-disclaimer.sh).
//
//go:embed disclaimer.md
var disclaimerMD string

// DisclaimerMarkdown returns the canonical Markdown text.
func DisclaimerMarkdown() string { return disclaimerMD }

// DisclaimerFull is the same text rendered as plain text for the dialog.
var DisclaimerFull = plainText(disclaimerMD)

var (
	italicRe = regexp.MustCompile(`\*([^*\n]+)\*`)
	boldRe   = regexp.MustCompile(`\*\*([^*\n]+)\*\*`)
)

// plainText turns the small Markdown subset used by the disclaimer
// (headings, bullets, bold, italics, code) into readable plain text.
func plainText(md string) string {
	var out []string
	for _, line := range strings.Split(strings.TrimSpace(md), "\n") {
		switch {
		case strings.HasPrefix(line, "# "):
			line = strings.ToUpper(strings.TrimPrefix(line, "# "))
		case strings.HasPrefix(line, "## "):
			line = strings.ToUpper(strings.TrimPrefix(line, "## "))
		case strings.HasPrefix(line, "* "):
			line = "- " + strings.TrimPrefix(line, "* ")
		}
		line = boldRe.ReplaceAllString(line, "$1")
		line = italicRe.ReplaceAllString(line, "$1")
		line = strings.ReplaceAll(line, "`", "")
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}
