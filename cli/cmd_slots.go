package main

import (
	"bytes"
	"database/sql"
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/bugraaktug/pgslot/cli/internal/format"
	"github.com/bugraaktug/pgslot/cli/internal/pg"
)

func renderSlotsTable(w io.Writer, slots []pg.SlotHealth) {
	var buf bytes.Buffer
	tw := tabwriter.NewWriter(&buf, 0, 4, 2, ' ', 0)
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
	// colorize after tabwriter has aligned columns on plain text -- see
	// format.StatusLabel/Colorize for why this can't happen the other way round.
	io.WriteString(w, format.Colorize(buf.String()))
}

func cmdSlots(db *sql.DB, w io.Writer) error {
	slots, err := pg.FetchSlotHealth(db)
	if err != nil {
		return err
	}
	renderSlotsTable(w, slots)
	return nil
}
