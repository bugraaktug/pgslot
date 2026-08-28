package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"strconv"
	"text/tabwriter"

	"github.com/bugraaktug/pgslot/cli/internal/format"
	"github.com/bugraaktug/pgslot/cli/internal/pg"
)

// metricField pulls a single key out of a slot_pipeline row's raw
// adapter_metrics JSON. Metrics are schema-free by design (see
// adapters/README.md) -- any key, or the whole blob, can legitimately be
// absent, so a missing key is not an error, just "n/a".
func metricField(raw []byte, key string) string {
	if raw == nil {
		return "n/a"
	}
	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil {
		return "n/a"
	}
	v, ok := m[key]
	if !ok || v == nil {
		return "n/a"
	}
	// encoding/json unmarshals every JSON number as float64, e.g.
	// events_per_sec computed as count/interval can come back as
	// 6.666666666666667 -- round for display rather than dumping full
	// float precision into the table.
	if f, ok := v.(float64); ok {
		if f == math.Trunc(f) {
			return strconv.FormatFloat(f, 'f', 0, 64)
		}
		return strconv.FormatFloat(f, 'f', 2, 64)
	}
	return fmt.Sprintf("%v", v)
}

func renderPipelineTable(w io.Writer, rows []pg.PipelineRow) {
	var buf bytes.Buffer
	tw := tabwriter.NewWriter(&buf, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "SLOT\tSTATE\tRETAINED\tADAPTER\tPROCESSED LSN\tEVENTS/SEC\tQUEUE\tSINK\tLAST REPORT")
	for _, r := range rows {
		retained := "n/a"
		if r.RetainedBytes.Valid {
			retained = format.Bytes(r.RetainedBytes.Int64)
		}
		adapter := "n/a"
		if r.AdapterName.Valid {
			adapter = r.AdapterName.String
		}
		lastReport := "n/a"
		if r.AdapterSampleAt.Valid {
			lastReport = r.AdapterSampleAt.Time.Format("2006-01-02 15:04:05")
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			r.SlotName, format.StatusLabel(r.Status), retained, adapter,
			metricField(r.AdapterMetrics, "processed_lsn"),
			metricField(r.AdapterMetrics, "events_per_sec"),
			metricField(r.AdapterMetrics, "queue_depth"),
			metricField(r.AdapterMetrics, "sink_status"),
			lastReport)
	}
	tw.Flush()
	// colorize after tabwriter has aligned columns on plain text -- see
	// format.StatusLabel/Colorize for why this can't happen the other way round.
	io.WriteString(w, format.Colorize(buf.String()))
}

func cmdPipeline(db *sql.DB, w io.Writer) error {
	rows, err := pg.FetchPipeline(db)
	if err != nil {
		return err
	}
	renderPipelineTable(w, rows)
	return nil
}
