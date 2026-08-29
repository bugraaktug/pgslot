package tui

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/bugraaktug/pgslot/cli/pg"
	"github.com/bugraaktug/pgslot/ui/internal/store"
)

func (a *App) buildTree() *tview.TreeView {
	a.treeRoot = tview.NewTreeNode("Connections").
		SetColor(tcell.ColorYellow).
		SetSelectable(true)
	a.rebuildTreeChildren()

	tree := tview.NewTreeView().
		SetRoot(a.treeRoot).
		SetCurrentNode(a.treeRoot)

	tree.SetSelectedFunc(func(node *tview.TreeNode) {
		c, ok := node.GetReference().(store.Connection)
		if !ok {
			node.SetExpanded(!node.IsExpanded())
			return
		}
		a.connectTo(c)
	})

	return tree
}

// rebuildTreeChildren re-derives the tree's child nodes from a.conn --
// tview's tree is retained-mode (you own the node list directly), so a
// connection added or removed just needs its node added/removed here, no
// separate "refresh" data-walk step to get right.
func (a *App) rebuildTreeChildren() {
	a.treeRoot.ClearChildren()
	for _, c := range a.conn {
		label := c.Name
		if a.active != nil && a.active.Name == c.Name {
			label += "  ●"
		}
		n := tview.NewTreeNode(label).
			SetReference(c).
			SetSelectable(true).
			SetColor(tcell.ColorWhite)
		a.treeRoot.AddChild(n)
	}
	a.treeRoot.SetExpanded(true)
}

func (a *App) selectedConnection() (store.Connection, bool) {
	node := a.tree.GetCurrentNode()
	if node == nil {
		return store.Connection{}, false
	}
	c, ok := node.GetReference().(store.Connection)
	return c, ok
}

func (a *App) connectTo(c store.Connection) {
	db, err := pg.Connect(c.DSN)
	if err != nil {
		a.showMessage(fmt.Sprintf("connect %s: %v", c.Name, err))
		return
	}
	a.disconnect()
	a.db = db
	active := c
	a.active = &active
	a.rebuildTreeChildren()
	a.showOverview()
}

func (a *App) removeSelectedConnection() {
	c, ok := a.selectedConnection()
	if !ok {
		return
	}
	a.confirm(fmt.Sprintf("Remove saved connection %q?", c.Name), func() {
		a.conn = store.Remove(a.conn, c.Name)
		if err := store.Save(a.conn); err != nil {
			a.showMessage(err.Error())
		}
		if a.active != nil && a.active.Name == c.Name {
			a.disconnect()
		}
		a.rebuildTreeChildren()
	})
}

// showAddConnectionForm shows a modal form over the main layout -- the
// guaranteed-reliable path to add a connection, independent of tree
// focus/keybinding edge cases.
func (a *App) showAddConnectionForm() {
	form := tview.NewForm()
	form.AddInputField("Name", "", 30, nil, nil)
	form.AddInputField("DSN", "", 60, nil, nil)
	form.AddButton("Add", func() {
		name := form.GetFormItemByLabel("Name").(*tview.InputField).GetText()
		dsn := form.GetFormItemByLabel("DSN").(*tview.InputField).GetText()
		if name == "" || dsn == "" {
			return
		}
		a.conn = store.Add(a.conn, store.Connection{Name: name, DSN: dsn})
		if err := store.Save(a.conn); err != nil {
			a.showMessage(err.Error())
		}
		a.rebuildTreeChildren()
		a.closeModal()
	})
	form.AddButton("Cancel", func() { a.closeModal() })
	form.SetBorder(true).SetTitle(" Add connection (Esc to cancel) ")
	form.SetCancelFunc(func() { a.closeModal() })

	a.showModal("add-connection", centered(form, 70, 9))
}

func (a *App) showModal(name string, p tview.Primitive) {
	a.root.AddPage(name, p, true, true)
	a.app.SetFocus(p)
}

func (a *App) closeModal() {
	name, _ := a.root.GetFrontPage()
	if name != "main" {
		a.root.RemovePage(name)
	}
	a.app.SetFocus(a.tree)
}

func (a *App) confirm(text string, onConfirm func()) {
	modal := tview.NewModal().
		SetText(text).
		AddButtons([]string{"Yes", "Cancel"}).
		SetDoneFunc(func(_ int, label string) {
			a.closeModal()
			if label == "Yes" {
				onConfirm()
			}
		})
	a.showModal("confirm", modal)
}

func (a *App) showMessage(text string) {
	modal := tview.NewModal().
		SetText(text).
		AddButtons([]string{"OK"}).
		SetDoneFunc(func(_ int, _ string) { a.closeModal() })
	a.showModal("message", modal)
}

// centered wraps p in a fixed-size box centered on screen -- tview has no
// built-in modal-with-custom-form primitive (Modal is buttons-only), so
// this is the standard Flex-of-spacers technique.
func centered(p tview.Primitive, width, height int) tview.Primitive {
	return tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(p, height, 0, true).
			AddItem(nil, 0, 1, false),
			width, 0, true).
		AddItem(nil, 0, 1, false)
}
