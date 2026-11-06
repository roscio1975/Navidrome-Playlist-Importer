package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"golang.org/x/term"
)

// ANSI styles used by the terminal output. They are only emitted when stdout
// is a terminal, NO_COLOR is not set and --no-color was not passed.
const (
	ansiReset   = "\033[0m"
	ansiBold    = "\033[1m"
	ansiDim     = "\033[2m"
	ansiRed     = "\033[31m"
	ansiGreen   = "\033[32m"
	ansiYellow  = "\033[33m"
	ansiCyan    = "\033[36m"
	ansiMagenta = "\033[35m"
)

var useColor bool

func colorEnabled(disabled bool) bool {
	if disabled {
		return false
	}
	if os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		return false
	}
	return term.IsTerminal(int(os.Stdout.Fd()))
}

func paint(code, s string) string {
	if !useColor || code == "" {
		return s
	}
	return code + s + ansiReset
}

func dim(s string) string    { return paint(ansiDim, s) }
func bold(s string) string   { return paint(ansiBold, s) }
func green(s string) string  { return paint(ansiGreen, s) }
func yellow(s string) string { return paint(ansiYellow, s) }
func red(s string) string    { return paint(ansiRed, s) }
func cyan(s string) string   { return paint(ansiCyan, s) }

// field prints an aligned "label  value" line.
func field(label, value string) {
	fmt.Printf("  %s  %s\n", paint(ansiBold, fmt.Sprintf("%-9s", label)), value)
}

// num formats an integer with thousands separators (9,284).
func num(n int) string {
	s := strconv.Itoa(n)
	neg := strings.HasPrefix(s, "-")
	if neg {
		s = s[1:]
	}
	var b strings.Builder
	for i, r := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(r)
	}
	if neg {
		return "-" + b.String()
	}
	return b.String()
}

// plural returns "1 row" or "N rows".
func plural(n int, one, many string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s", one)
	}
	return fmt.Sprintf("%d %s", n, many)
}

// statusStyle returns symbol, short label and color for a status.
func statusStyle(status string) (string, string, string) {
	switch status {
	case StatusExact:
		return "✓", "EXACT", ansiGreen
	case StatusAutomatic:
		return "≈", "AUTOMATIC", ansiCyan
	case StatusManual:
		return "★", "MANUAL", ansiMagenta
	case StatusAmbiguousFirst, StatusAmbiguousSkip:
		return "?", "AMBIGUOUS", ansiYellow
	case StatusNotFound:
		return "✗", "NOT FOUND", ansiRed
	case StatusSkipped:
		return "–", "SKIPPED", ansiDim
	case StatusDuplicate:
		return "=", "DUPLICATE", ansiDim
	case StatusDiscarded:
		return "−", "DISCARDED", ansiDim
	default:
		return "·", status, ""
	}
}

// statusTag is the fixed-width, colored badge printed before each track.
func statusTag(status string) string {
	sym, label, code := statusStyle(status)
	return paint(code, fmt.Sprintf("%s %-13s", sym, label))
}

// progressTag is the dim "12/75" prefix.
func progressTag(i, total int) string {
	return dim(fmt.Sprintf("%9s", fmt.Sprintf("%d/%d", i, total)))
}

// describeEntry returns the best available one-line description for an entry.
func describeEntry(e Entry) string {
	if e.Song != nil {
		return formatSong(e.Song)
	}
	return describeRow(e.Row)
}

// describeRow returns the CSV row as «title» — artist [ album ].
func describeRow(r Row) string {
	var b strings.Builder
	b.WriteString("«")
	b.WriteString(r.Track)
	b.WriteString("»")
	if r.Artist != "" {
		b.WriteString(" — ")
		b.WriteString(r.Artist)
	}
	if r.Album != "" {
		b.WriteString(" [ ")
		b.WriteString(r.Album)
		b.WriteString(" ]")
	}
	return b.String()
}

// trackLine prints one resolved CSV row in the compact log format.
func trackLine(i, total int, e Entry) {
	fmt.Printf("  %s  %s %s\n", progressTag(i, total), statusTag(e.Status), describeEntry(e))
}
