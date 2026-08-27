package main

import (
	"database/sql"
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/bugraaktug/pgslot/cli/internal/format"
	"github.com/bugraaktug/pgslot/cli/internal/pg"
)

func renderSlotsTable(w io.Writer, slots []pg.SlotHealth) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "SLOT\tSTATE\tWAL DISTANCE\tRATE")
	for _, s := range slots {
		dist := "n/a"
		if s.RetainedBytes.Valid {
			dist = format.Bytes(s.RetainedBytes.Int64)
		}
		rate := "n/a"
		if s.WalGrowth.Valid && s.ConsumerRate.Valid {
			rate = format.Rate(s.WalGrowth.Float64 - s.ConsumerRate.Float64)
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", s.Name, format.StatusLabel(s.Status), dist, rate)
	}
	tw.Flush()
}

func cmdSlots(db *sql.DB, w io.Writer) error {
	slots, err := pg.FetchSlotHealth(db)
	if err != nil {
		return err
	}
	renderSlotsTable(w, slots)
	return nil
}
