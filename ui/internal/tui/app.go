// Package tui is pgslot-ui's terminal interface: a pgAdmin-style layout
// (connections tree on the left, tabbed content on the right) built with
// tview. All database access goes through cli/pg's existing
// Connect/Fetch* functions -- no query code is duplicated here.
package tui

import (
	"database/sql"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/bugraaktug/pgslot/cli/pg"
	"github.com/bugraaktug/pgslot/ui/internal/store"
)

// pollInterval matches the CLI's `watch -interval` default.
const pollInterval = 2 * time.Second

type App struct {
	app  *tview.Application
	root *tview.Pages // top-level: "main" vs modal pages (add-connection)

	tree     *tview.TreeView
	treeRoot *tview.TreeNode

	conn []store.Connection

	active *store.Connection
	db     *sql.DB

	pollStop chan struct{}

	content           *tview.Pages // right-hand pages: overview, publications, detail:<slot>
	overviewTable     *tview.Table
	publicationsTable *tview.Table
	summaryView       *tview.TextView
	pipeline          []pg.PipelineRow
	openDetailPages   map[string]bool

	status *tview.TextView
}

func New() *App {
	conns, _ := store.Load()
	return &App{
		app:             tview.NewApplication(),
		conn:            conns,
		openDetailPages: map[string]bool{},
	}
}

func (a *App) Run() error {
	a.buildLayout()
	a.app.EnableMouse(true)
	a.app.SetRoot(a.root, true)
	return a.app.Run()
}

func (a *App) buildLayout() {
	a.tree = a.buildTree()

	a.content = tview.NewPages()
	a.overviewTable = a.buildOverviewTable()
	a.summaryView = tview.NewTextView().SetDynamicColors(true)
	overviewFlex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(a.summaryView, 1, 0, false).
		AddItem(a.overviewTable, 0, 1, true)
	a.content.AddPage("overview", overviewFlex, true, true)
	a.content.AddPage("publications", a.buildPublicationsTable(), true, false)

	a.status = tview.NewTextView().SetDynamicColors(true).
		SetText("[::b]a[::-]dd  [::b]d[::-]elete  [::b]enter[::-] connect/open  [::b]tab[::-] switch pane  " +
			"[::b]1[::-] overview  [::b]2[::-] publications  [::b]esc[::-] close tab  [::b]q[::-]uit")

	treeBox := a.tree
	treeBox.SetBorder(true).SetTitle(" Connections ")
	a.content.SetBorder(true).SetTitle(" pgslot ")

	body := tview.NewFlex().
		AddItem(treeBox, 32, 0, true).
		AddItem(a.content, 0, 1, false)

	main := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(body, 0, 1, true).
		AddItem(a.status, 1, 0, false)

	a.root = tview.NewPages().AddPage("main", main, true, true)

	a.app.SetInputCapture(a.handleGlobalKeys)
}

func (a *App) handleGlobalKeys(event *tcell.EventKey) *tcell.EventKey {
	// Modal pages (add-connection form) handle their own keys (Esc to
	// cancel) -- don't let global shortcuts like 'a'/'d' leak through
	// while a form is focused and the user is typing.
	if front, _ := a.root.GetFrontPage(); front != "main" {
		return event
	}
	switch event.Key() {
	case tcell.KeyTab:
		if a.app.GetFocus() == a.tree {
			a.app.SetFocus(a.content)
		} else {
			a.app.SetFocus(a.tree)
		}
		return nil
	case tcell.KeyEscape:
		a.closeCurrentDetailTab()
		return nil
	}
	switch event.Rune() {
	case 'q':
		a.app.Stop()
		return nil
	case 'a':
		if a.app.GetFocus() == a.tree {
			a.showAddConnectionForm()
			return nil
		}
	case 'd':
		if a.app.GetFocus() == a.tree {
			a.removeSelectedConnection()
			return nil
		}
	case '1':
		a.content.SwitchToPage("overview")
		return nil
	case '2':
		if a.db != nil {
			a.content.SwitchToPage("publications")
		}
		return nil
	}
	return event
}

func (a *App) startPoll() {
	a.stopPoll()
	a.pollStop = make(chan struct{})
	stop := a.pollStop
	go func() {
		ticker := time.NewTicker(pollInterval)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				a.refreshOverview()
			}
		}
	}()
}

func (a *App) stopPoll() {
	if a.pollStop != nil {
		close(a.pollStop)
		a.pollStop = nil
	}
}

// disconnect closes the active DB connection and stops polling, without
// touching the tree or any open detail tabs -- the user closes those
// explicitly (Esc).
func (a *App) disconnect() {
	a.stopPoll()
	if a.db != nil {
		a.db.Close()
		a.db = nil
	}
	a.active = nil
}
