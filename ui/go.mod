module github.com/bugraaktug/pgslot/ui

go 1.21

require (
	github.com/bugraaktug/pgslot/cli v0.0.0-00010101000000-000000000000
	github.com/gdamore/tcell/v2 v2.8.1
	github.com/rivo/tview v0.42.0
)

require (
	github.com/gdamore/encoding v1.0.1 // indirect
	github.com/lib/pq v1.10.9 // indirect
	github.com/lucasb-eyer/go-colorful v1.2.0 // indirect
	github.com/mattn/go-runewidth v0.0.16 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	golang.org/x/sys v0.29.0 // indirect
	golang.org/x/term v0.28.0 // indirect
	golang.org/x/text v0.21.0 // indirect
)

// go.work's workspace `use` mechanism should resolve this locally without
// needing this replace directive on a current Go toolchain, but it's kept
// as a version-independent fallback (see git history for why).
replace github.com/bugraaktug/pgslot/cli => ../cli
