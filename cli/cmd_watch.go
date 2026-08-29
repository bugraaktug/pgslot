package main

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bugraaktug/pgslot/cli/pg"
)

const clearScreen = "\033[H\033[2J"

func cmdWatch(db *sql.DB, w io.Writer, interval time.Duration) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		slots, err := pg.FetchSlotHealth(db)
		if err != nil {
			return err
		}
		fmt.Fprint(w, clearScreen)
		fmt.Fprintf(w, "pgslot watch -- refreshing every %s, ctrl-c to quit\n\n", interval)
		renderSlotsTable(w, slots)
		fmt.Fprintf(w, "\nlast refresh: %s\n", time.Now().Format(time.RFC3339))

		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}
