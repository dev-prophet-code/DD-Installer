package ui

import (
	"regexp"
	"strconv"
)

var progressRe = regexp.MustCompile(`(?i)progress:\s*([0-9]{1,3}(?:\.[0-9]+)?)`)
var percentRe = regexp.MustCompile(`(?i)\b([0-9]{1,3}(?:\.[0-9]+)?)\s?%`)

// parseProgress extracts a percentage (0-100) from a log line such as
// SteamCMD's "Update state (0x61) downloading, progress: 45.12 (a / b)" or a
// generic "37%". ok is false for lines without one.
func parseProgress(line string) (pct float64, ok bool) {
	for _, re := range []*regexp.Regexp{progressRe, percentRe} {
		if m := re.FindStringSubmatch(line); m != nil {
			v, err := strconv.ParseFloat(m[1], 64)
			if err == nil && v >= 0 && v <= 100 {
				return v, true
			}
		}
	}
	return 0, false
}
