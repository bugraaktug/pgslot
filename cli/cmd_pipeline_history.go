package main

import (
	"database/sql"
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/bugraaktug/pgslot/cli/pg"
)

func cmdPipelineHistory(db *sql.DB, w io.Writer, slotName string, limit int) error {
	points, err := pg.FetchPipelineHistory(db, slotName, limit)
	if err != nil {
		return err
	}
	if len(points) == 0 {
		fmt.Fprintf(w, "no adapter-reported metrics for slot %q\n", slotName)
		return nil
	}

	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "COLLECTED AT\tADAPTER\tPROCESSED LSN\tEVENTS/SEC\tQUEUE\tSINK")
	// points arrive newest-first; print oldest-first so the trend reads top-to-bottom,
	// matching cmdHistory's convention.
	for i := len(points) - 1; i >= 0; i-- {
		p := points[i]
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n",
			p.CollectedAt.Format("2006-01-02 15:04:05"), p.AdapterName,
			metricField(p.Metrics, "processed_lsn"),
			metricField(p.Metrics, "events_per_sec"),
			metricField(p.Metrics, "queue_depth"),
			metricField(p.Metrics, "sink_status"))
	}
	tw.Flush()
	return nil
}
