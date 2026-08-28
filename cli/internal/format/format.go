// Package format renders pgslot values (bytes, rates, status) for terminal output.
package format

import (
	"fmt"
	"os"
	"strings"
)

var NoColor = os.Getenv("NO_COLOR") != ""

const (
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorRed    = "\033[31m"
	colorReset  = "\033[0m"
)

// Bytes renders a byte count as e.g. "1.2 GB", matching the CLI's table style.
func Bytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for v := n / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	units := []string{"KB", "MB", "GB", "TB", "PB"}
	return fmt.Sprintf("%.1f %s", float64(n)/float64(div), units[exp])
}

// Rate renders bytes/sec net of consumption as a signed rate, e.g. "-12 MB/s", "+43 MB/s".
// netBytesPerSec > 0 means WAL is growing faster than it's consumed (retention increasing).
func Rate(netBytesPerSec float64) string {
	sign := "+"
	v := netBytesPerSec
	if v < 0 {
		sign = "-"
		v = -v
	}
	abs := Bytes(int64(v))
	return sign + abs + "/s"
}

// StatusLabel maps pgslot's status column (ok/warning/critical) to the CLI's
// plain HEALTHY/WARNING/CRITICAL display label -- no color. Use this inside
// anything that goes through text/tabwriter: tabwriter counts raw bytes,
// including invisible ANSI escapes, when computing column widths, so a
// colored cell reports as wider than it looks and throws off alignment
// against uncolored cells (e.g. the header) in the same column. Colorize
// the fully-aligned output afterward with Colorize instead.
func StatusLabel(status string) string {
	switch status {
	case "ok":
		return "HEALTHY"
	case "warning":
		return "WARNING"
	case "critical":
		return "CRITICAL"
	default:
		return status
	}
}

// Colorize wraps HEALTHY/WARNING/CRITICAL in their status colors within
// already-rendered (and thus already correctly column-aligned) text. Must
// run after tabwriter.Flush, never before -- see StatusLabel.
func Colorize(rendered string) string {
	if NoColor {
		return rendered
	}
	for label, color := range map[string]string{
		"HEALTHY":  colorGreen,
		"WARNING":  colorYellow,
		"CRITICAL": colorRed,
	} {
		rendered = strings.ReplaceAll(rendered, label, color+label+colorReset)
	}
	return rendered
}
