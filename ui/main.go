// Command pgslot-ui is a read-only terminal UI over the pgslot Postgres
// extension's views. Same read-only contract as the extension and CLI --
// never mutates a slot.
package main

import (
	"fmt"
	"os"

	"github.com/bugraaktug/pgslot/ui/internal/tui"
)

func main() {
	if err := tui.New().Run(); err != nil {
		fmt.Fprintf(os.Stderr, "pgslot-ui: %v\n", err)
		os.Exit(1)
	}
}
