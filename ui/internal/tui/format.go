package tui

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"

	"github.com/bugraaktug/pgslot/cli/pg"
)

func formatBytes(n int64) string {
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

func statusColor(status string) tcell.Color {
	switch status {
	case "ok":
		return tcell.ColorGreen
	case "warning":
		return tcell.ColorYellow
	case "critical":
		return tcell.ColorRed
	default:
		return tcell.ColorWhite
	}
}

func statusLabelText(status string) string {
	return strings.ToUpper(status)
}

// statusDot is a single-character "light" for the Overview table's
// rightmost column, colored via the cell's TextColor (Table cells don't
// parse [color] tags the way TextView does -- see the SetTextColor call at
// the call site). Tried as a full-row background tint first, but that
// dominated the table when most rows were critical (a common real case:
// inactive slots retaining WAL). A single-character indicator flags the
// row without overwhelming it, and leaves a natural spot for a future
// per-row action.
const statusDot = "●"


// activeLabelText is a separate fact from STATUS on purpose: a slot can be
// CRITICAL while still active, or CRITICAL because it's inactive and
// retaining WAL -- the two are independent facts, not implied by each
// other.
func activeLabelText(active bool) string {
	if active {
		return "yes"
	}
	return "no"
}

func boolLabel(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

func nullString(s string, valid bool) string {
	if !valid {
		return "n/a"
	}
	return s
}

// formatDashboard renders an htop-style live header: cluster totals plus a
// per-status breakdown and a "last updated" clock -- the visible proof
// this view is actively polling (every pollInterval), not a static
// snapshot the user has to manually refresh.
func formatDashboard(s pg.WalSummary, rows []pg.PipelineRow, updatedAt time.Time) string {
	retained := "n/a"
	if s.TotalRetained.Valid {
		retained = formatBytes(s.TotalRetained.Int64)
	}
	var ok, warn, crit int
	for _, r := range rows {
		switch r.Status {
		case "ok":
			ok++
		case "warning":
			warn++
		case "critical":
			crit++
		}
	}
	return fmt.Sprintf(" %d slots (%d active) - %s retained  |  [green]OK:%d[-] [yellow]WARN:%d[-] [red]CRIT:%d[-]  |  updated %s",
		s.SlotCount, s.ActiveCount, retained, ok, warn, crit, updatedAt.Format("15:04:05"))
}

// metricField pulls a single key out of a slot_pipeline row's raw
// adapter_metrics JSON, mirroring the CLI's cmd_pipeline.go metricField.
// Metrics are schema-free by design (see adapters/README.md) -- a missing
// key is not an error, just "n/a".
func metricField(raw []byte, key string) string {
	if raw == nil {
		return "n/a"
	}
	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil {
		return "n/a"
	}
	v, ok := m[key]
	if !ok || v == nil {
		return "n/a"
	}
	if f, ok := v.(float64); ok {
		if f == math.Trunc(f) {
			return strconv.FormatFloat(f, 'f', 0, 64)
		}
		return strconv.FormatFloat(f, 'f', 2, 64)
	}
	return fmt.Sprintf("%v", v)
}

// metricFloat is metricField's numeric counterpart, used by the sparkline
// series in detail.go.
func metricFloat(raw []byte, key string) (float64, bool) {
	if raw == nil {
		return 0, false
	}
	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil {
		return 0, false
	}
	v, ok := m[key]
	if !ok || v == nil {
		return 0, false
	}
	f, ok := v.(float64)
	return f, ok
}

var sparkBlocks = []rune("▁▂▃▄▅▆▇█")

// sparkline renders values as a compact block-character trend line -- the
// terminal-appropriate substitute for a line chart.
func sparkline(values []float64) string {
	if len(values) == 0 {
		return "(no data)"
	}
	min, max := values[0], values[0]
	for _, v := range values {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}
	var b strings.Builder
	for _, v := range values {
		idx := 0
		if max > min {
			idx = int((v - min) / (max - min) * float64(len(sparkBlocks)-1))
		}
		if idx < 0 {
			idx = 0
		}
		if idx >= len(sparkBlocks) {
			idx = len(sparkBlocks) - 1
		}
		b.WriteRune(sparkBlocks[idx])
	}
	fmt.Fprintf(&b, "  (min %.0f, max %.0f)", min, max)
	return b.String()
}
