package tui

import (
	"fmt"
	"strings"

	"github.com/rivo/tview"

	"github.com/bugraaktug/pgslot/cli/pg"
)

const detailHistoryLimit = 50

// openDetailTab fetches slot_history_rates and slot_pipeline_history for
// slotName once (not polled -- same reasoning as the original GUI design:
// don't pull full history for every slot on every overview poll tick) and
// opens/switches to a page for it. Reuses the page if already open.
func (a *App) openDetailTab(slotName string) {
	pageName := "detail:" + slotName
	if a.openDetailPages[pageName] {
		a.content.SwitchToPage(pageName)
		return
	}

	history, err := pg.FetchHistory(a.db, slotName, detailHistoryLimit)
	if err != nil {
		history = nil
	}
	pipelineHistory, err := pg.FetchPipelineHistory(a.db, slotName, detailHistoryLimit)
	if err != nil {
		pipelineHistory = nil
	}

	view := tview.NewTextView().
		SetDynamicColors(true).
		SetText(renderDetail(slotName, history, pipelineHistory))
	view.SetBorder(true).SetTitle(fmt.Sprintf(" %s (Esc to close) ", slotName))

	a.content.AddPage(pageName, view, true, true)
	a.openDetailPages[pageName] = true
	a.content.SwitchToPage(pageName)
}

func (a *App) closeCurrentDetailTab() {
	name, _ := a.content.GetFrontPage()
	if name == "overview" || name == "publications" {
		return
	}
	a.content.RemovePage(name)
	delete(a.openDetailPages, name)
	a.content.SwitchToPage("overview")
}

// renderDetail builds two independent time series -- WAL growth vs.
// consumer rate (slot_history_rates), and adapter events/sec + queue depth
// (slot_pipeline_history) -- each with a compact sparkline. These run on
// independent time axes (adapter report ticks vs. collect() ticks are
// scheduled separately), so they're shown separately, not merged into one
// timeline.
func renderDetail(slotName string, history []pg.HistoryPoint, pipelineHistory []pg.PipelineHistoryPoint) string {
	var b strings.Builder

	b.WriteString("[yellow::b]WAL growth vs. consumer rate[-::-] (slot_history_rates)\n\n")
	var growth, consumer []float64
	for i := len(history) - 1; i >= 0; i-- {
		p := history[i]
		if p.WalGrowth.Valid {
			growth = append(growth, p.WalGrowth.Float64)
		}
		if p.ConsumerRate.Valid {
			consumer = append(consumer, p.ConsumerRate.Float64)
		}
	}
	if len(growth) == 0 && len(consumer) == 0 {
		b.WriteString("  not enough data yet (needs at least two collect() snapshots)\n")
	} else {
		fmt.Fprintf(&b, "  WAL growth B/s : %s\n", sparkline(growth))
		fmt.Fprintf(&b, "  Consumer   B/s : %s\n", sparkline(consumer))
	}

	b.WriteString("\n[yellow::b]Recent snapshots[-::-]\n\n")
	b.WriteString("  COLLECTED AT          ACTIVE  WAL_STATUS   RETAINED    GROWTH B/s   CONSUMER B/s\n")
	limit := 15
	for i, p := range history {
		if i >= limit {
			break
		}
		retained := "n/a"
		if p.RetainedBytes.Valid {
			retained = formatBytes(p.RetainedBytes.Int64)
		}
		growthStr := "n/a"
		if p.WalGrowth.Valid {
			growthStr = fmt.Sprintf("%.0f", p.WalGrowth.Float64)
		}
		consumerStr := "n/a"
		if p.ConsumerRate.Valid {
			consumerStr = fmt.Sprintf("%.0f", p.ConsumerRate.Float64)
		}
		fmt.Fprintf(&b, "  %-20s   %-6s  %-11s  %-10s  %-11s  %s\n",
			p.CollectedAt.Format("2006-01-02 15:04:05"), activeLabelText(p.Active),
			nullString(p.WalStatus.String, p.WalStatus.Valid), retained, growthStr, consumerStr)
	}

	b.WriteString("\n[yellow::b]Adapter events/sec and queue depth[-::-] (slot_pipeline_history)\n\n")
	var events, queue []float64
	for i := len(pipelineHistory) - 1; i >= 0; i-- {
		p := pipelineHistory[i]
		if v, ok := metricFloat(p.Metrics, "events_per_sec"); ok {
			events = append(events, v)
		}
		if v, ok := metricFloat(p.Metrics, "queue_depth"); ok {
			queue = append(queue, v)
		}
	}
	if len(events) == 0 && len(queue) == 0 {
		b.WriteString("  no adapter reports for this slot yet\n")
	} else {
		fmt.Fprintf(&b, "  events/sec     : %s\n", sparkline(events))
		fmt.Fprintf(&b, "  queue depth    : %s\n", sparkline(queue))
	}

	b.WriteString("\n[yellow::b]Recent adapter reports[-::-]\n\n")
	b.WriteString("  COLLECTED AT          ADAPTER    EVENTS/SEC  QUEUE\n")
	for i, p := range pipelineHistory {
		if i >= limit {
			break
		}
		fmt.Fprintf(&b, "  %-20s   %-9s  %-10s  %s\n",
			p.CollectedAt.Format("2006-01-02 15:04:05"), p.AdapterName,
			metricField(p.Metrics, "events_per_sec"), metricField(p.Metrics, "queue_depth"))
	}

	return b.String()
}
