package tui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/bugraaktug/pgslot/cli/pg"
)

var publicationColumns = []string{"PUBLICATION", "OWNER", "ALL TABLES", "INSERT", "UPDATE", "DELETE", "TRUNCATE", "VIA ROOT"}

// publicationColumnsToExpand mirrors overviewColumnsToExpand's reasoning:
// the two text columns absorb unused table width instead of leaving it
// empty to the right of the table.
var publicationColumnsToExpand = map[int]bool{0: true, 1: true}

// buildPublicationsTable returns the (initially empty) Publications table
// widget; refreshPublicationsSync fills it in once per connect.
// pgslot.available_publications is informational only -- can't be joined
// to slot health since pg_replication_slots doesn't record which
// publication a slot consumes -- so this is a standalone table, not polled.
func (a *App) buildPublicationsTable() *tview.Table {
	t := tview.NewTable().
		SetFixed(1, 0).
		SetSelectable(false, false)
	for col, title := range publicationColumns {
		c := tview.NewTableCell(title).
			SetSelectable(false).
			SetTextColor(tcell.ColorYellow).
			SetAttributes(tcell.AttrBold)
		if publicationColumnsToExpand[col] {
			c.SetExpansion(1)
		}
		t.SetCell(0, col, c)
	}
	a.publicationsTable = t
	return t
}

func (a *App) refreshPublicationsSync() {
	t := a.publicationsTable
	for row := t.GetRowCount() - 1; row >= 1; row-- {
		t.RemoveRow(row)
	}
	pubs, err := pg.FetchAvailablePublications(a.db)
	if err != nil {
		t.SetCell(1, 0, tview.NewTableCell("error: "+err.Error()))
		return
	}
	for i, p := range pubs {
		row := i + 1
		t.SetCell(row, 0, tview.NewTableCell(p.Name))
		t.SetCell(row, 1, tview.NewTableCell(p.Owner))
		t.SetCell(row, 2, tview.NewTableCell(boolLabel(p.AllTables)))
		t.SetCell(row, 3, tview.NewTableCell(boolLabel(p.Insert)))
		t.SetCell(row, 4, tview.NewTableCell(boolLabel(p.Update)))
		t.SetCell(row, 5, tview.NewTableCell(boolLabel(p.Delete)))
		t.SetCell(row, 6, tview.NewTableCell(boolLabel(p.Truncate)))
		t.SetCell(row, 7, tview.NewTableCell(boolLabel(p.ViaRoot)))
	}
}
