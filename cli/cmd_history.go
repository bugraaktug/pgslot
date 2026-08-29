package main

import (
	"database/sql"
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/bugraaktug/pgslot/cli/internal/format"
	"github.com/bugraaktug/pgslot/cli/pg"
)

func cmdHistory(db *sql.DB, w io.Writer, slotName string, limit int) error {
	points, err := pg.FetchHistory(db, slotName, limit)
	if err != nil {
		return err
	}
	if len(points) == 0 {
		fmt.Fprintf(w, "no history for slot %q\n", slotName)
		return nil
	}

	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "COLLECTED AT\tACTIVE\tWAL STATUS\tRETAINED\tGROWTH\tCONSUMED")
	// points arrive newest-first; print oldest-first so the trend reads top-to-bottom.
	for i := len(points) - 1; i >= 0; i-- {
		p := points[i]
		retained, growth, consumed := "n/a", "n/a", "n/a"
		if p.RetainedBytes.Valid {
			retained = format.Bytes(p.RetainedBytes.Int64)
		}
		if p.WalGrowth.Valid {
			growth = format.Rate(p.WalGrowth.Float64)
		}
		if p.ConsumerRate.Valid {
			consumed = format.Rate(p.ConsumerRate.Float64)
		}
		walStatus := "?"
		if p.WalStatus.Valid {
			walStatus = p.WalStatus.String
		}
		fmt.Fprintf(tw, "%s\t%t\t%s\t%s\t%s\t%s\n",
			p.CollectedAt.Format("2006-01-02 15:04:05"), p.Active, walStatus, retained, growth, consumed)
	}
	tw.Flush()
	return nil
}
