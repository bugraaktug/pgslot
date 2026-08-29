package tui

import (
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/bugraaktug/pgslot/cli/pg"
)

// ADAPTER is deliberately labeled "latest" -- it's whichever adapter last
// called report_metric() for that slot (Walkrie today, but the contract in
// adapters/README.md is generic: any pgslot_adapter member can report),
// and slot_pipeline's LATERAL join picks only the most recent row per
// slot. A slot could in principle have reports from more than one adapter
// over time; this column never shows more than the latest one.
var overviewColumns = []string{"SLOT", "STATUS", "ACTIVE", "RETAINED", "ADAPTER (latest)", "EVENTS/SEC", "QUEUE", ""}

// columnsToExpand absorb the table's unused width (tview sizes columns to
// their content by default, which otherwise leaves a wide terminal mostly
// empty to the right of a narrow table) -- text-heavy columns are the
// natural place to grow, not the short numeric ones.
var columnsToExpand = map[int]bool{0: true, 4: true}

func (a *App) buildOverviewTable() *tview.Table {
	t := tview.NewTable().
		SetFixed(1, 0).
		SetSelectable(true, false)
	for col, title := range overviewColumns {
		c := tview.NewTableCell(title).
			SetSelectable(false).
			SetTextColor(tcell.ColorYellow).
			SetAttributes(tcell.AttrBold)
		if columnsToExpand[col] {
			c.SetExpansion(1)
		}
		t.SetCell(0, col, c)
	}
	t.SetSelectedFunc(func(row, col int) {
		if row < 1 || row-1 >= len(a.pipeline) {
			return
		}
		a.openDetailTab(a.pipeline[row-1].SlotName)
	})
	return t
}

// showOverview fetches slot_pipeline + wal_summary for the active
// connection, switches to the Overview page, and starts the 2s refresh
// poll. Publications is refreshed once per connect (not polled) since it's
// informational, not live pipeline state.
func (a *App) showOverview() {
	if a.db == nil {
		return
	}
	a.refreshOverviewSync()
	a.refreshPublicationsSync()
	a.content.SwitchToPage("overview")
	a.startPoll()
}

func (a *App) refreshOverviewSync() {
	rows, err := pg.FetchPipeline(a.db)
	if err != nil {
		a.summaryView.SetText("[red]" + err.Error())
		return
	}
	summary, _ := pg.FetchWalSummary(a.db)
	a.pipeline = rows
	a.redrawOverviewTable()
	a.summaryView.SetText(formatDashboard(summary, rows, time.Now()))
}

// refreshOverview is called from the poll goroutine -- must hop back onto
// the UI goroutine via QueueUpdateDraw before touching any tview widget.
// The htop-style "updated HH:MM:SS" stamp in formatDashboard is the visible
// proof this is a live, auto-refreshing view, not a static snapshot.
func (a *App) refreshOverview() {
	if a.db == nil {
		return
	}
	rows, err := pg.FetchPipeline(a.db)
	if err != nil {
		return
	}
	summary, err := pg.FetchWalSummary(a.db)
	if err != nil {
		return
	}
	now := time.Now()
	a.app.QueueUpdateDraw(func() {
		a.pipeline = rows
		a.redrawOverviewTable()
		a.summaryView.SetText(formatDashboard(summary, rows, now))
	})
}

func (a *App) redrawOverviewTable() {
	t := a.overviewTable
	for row := t.GetRowCount() - 1; row >= 1; row-- {
		t.RemoveRow(row)
	}
	for i, r := range a.pipeline {
		row := i + 1
		retained := "n/a"
		if r.RetainedBytes.Valid {
			retained = formatBytes(r.RetainedBytes.Int64)
		}
		t.SetCell(row, 0, tview.NewTableCell(r.SlotName))
		t.SetCell(row, 1, tview.NewTableCell(statusLabelText(r.Status)).SetTextColor(statusColor(r.Status)))
		t.SetCell(row, 2, tview.NewTableCell(activeLabelText(r.Active)).SetTextColor(tcell.ColorWhite))
		t.SetCell(row, 3, tview.NewTableCell(retained))
		t.SetCell(row, 4, tview.NewTableCell(nullString(r.AdapterName.String, r.AdapterName.Valid)))
		t.SetCell(row, 5, tview.NewTableCell(metricField(r.AdapterMetrics, "events_per_sec")))
		t.SetCell(row, 6, tview.NewTableCell(metricField(r.AdapterMetrics, "queue_depth")))
		// A colored status "light" on the right, separate from the STATUS
		// text column -- reserved as a spot for a future per-row action
		// (e.g. a keybinding to act on the selected row), not wired to
		// anything yet.
		t.SetCell(row, 7, tview.NewTableCell(statusDot).
			SetTextColor(statusColor(r.Status)).
			SetAlign(tview.AlignCenter))
	}
}
