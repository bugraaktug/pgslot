package main

import (
	"database/sql"
	"fmt"
	"io"

	"github.com/bugraaktug/pgslot/cli/internal/format"
	"github.com/bugraaktug/pgslot/cli/pg"
)

func cmdHealth(db *sql.DB, w io.Writer) error {
	summary, err := pg.FetchWalSummary(db)
	if err != nil {
		return err
	}
	slots, err := pg.FetchSlotHealth(db)
	if err != nil {
		return err
	}

	var ok, warn, crit int
	for _, s := range slots {
		switch s.Status {
		case "ok":
			ok++
		case "warning":
			warn++
		case "critical":
			crit++
		}
	}

	fmt.Fprintf(w, "%d slot(s): %d healthy, %d warning, %d critical\n", summary.SlotCount, ok, warn, crit)
	fmt.Fprintf(w, "%d active\n", summary.ActiveCount)
	if summary.TotalRetained.Valid {
		fmt.Fprintf(w, "total retained: %s\n", format.Bytes(summary.TotalRetained.Int64))
	}
	if summary.TopConsumerSlot.Valid {
		fmt.Fprintf(w, "top consumer: %s (%s)\n", summary.TopConsumerSlot.String, format.Bytes(summary.MaxRetained.Int64))
	}
	return nil
}
