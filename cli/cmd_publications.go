package main

import (
	"bytes"
	"database/sql"
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/bugraaktug/pgslot/cli/pg"
)

func boolLabel(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

func renderPublicationsTable(w io.Writer, rows []pg.AvailablePublication) {
	var buf bytes.Buffer
	tw := tabwriter.NewWriter(&buf, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "PUBLICATION\tOWNER\tALL TABLES\tINSERT\tUPDATE\tDELETE\tTRUNCATE\tVIA ROOT")
	for _, p := range rows {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			p.Name, p.Owner, boolLabel(p.AllTables), boolLabel(p.Insert),
			boolLabel(p.Update), boolLabel(p.Delete), boolLabel(p.Truncate),
			boolLabel(p.ViaRoot))
	}
	tw.Flush()
	io.WriteString(w, buf.String())
}

func cmdPublications(db *sql.DB, w io.Writer) error {
	rows, err := pg.FetchAvailablePublications(db)
	if err != nil {
		return err
	}
	renderPublicationsTable(w, rows)
	return nil
}
