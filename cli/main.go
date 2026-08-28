// Command pgslot is a read-only CLI over the pgslot Postgres extension's
// views (slot_health, slot_rates, slot_history_rates, wal_summary). It
// never mutates a slot -- same read-only contract as the extension itself.
package main

import (
	"database/sql"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/bugraaktug/pgslot/cli/internal/pg"
)

// known flag names for every subcommand, used by reorderFlags.
var knownFlags = map[string]bool{"dsn": true, "interval": true, "n": true}

// reorderFlags moves recognized -flag [value] pairs to the front of args so
// flag.Parse (which stops at the first non-flag token) still sees them
// regardless of where the user typed them relative to positional args.
func reorderFlags(args []string) []string {
	var flags, positional []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		name := strings.TrimLeft(a, "-")
		if strings.HasPrefix(a, "-") && knownFlags[strings.SplitN(name, "=", 2)[0]] {
			flags = append(flags, a)
			if !strings.Contains(a, "=") && i+1 < len(args) {
				i++
				flags = append(flags, args[i])
			}
			continue
		}
		positional = append(positional, a)
	}
	return append(flags, positional...)
}

const usage = `pgslot - replication slot WAL health, history, and diagnostics

Usage:
  pgslot health              cluster-wide summary
  pgslot slots                per-slot status table
  pgslot watch [-interval N]  auto-refreshing status table
  pgslot history <slot> [-n N]  per-slot snapshot history
  pgslot pipeline             per-slot status joined with adapter-reported metrics
  pgslot pipeline-history <slot> [-n N]  per-slot adapter-reported metrics history

Connection: set standard PG* env vars (PGHOST, PGUSER, PGDATABASE, ...),
same as psql, or pass -dsn / set PGSLOT_DSN.
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(1)
	}

	cmd := os.Args[1]
	fs := flag.NewFlagSet(cmd, flag.ExitOnError)
	dsn := fs.String("dsn", os.Getenv("PGSLOT_DSN"), "Postgres connection string (default: PG* env vars)")
	interval := fs.Duration("interval", 2*time.Second, "refresh interval for watch")
	n := fs.Int("n", 20, "number of history rows for history")
	// flag stops at the first non-flag arg, but "pgslot history <slot> -n 5"
	// puts the positional slot name first -- reorder flags to the front so
	// they're not silently left unparsed regardless of where they appear.
	fs.Parse(reorderFlags(os.Args[2:]))

	db, err := pg.Connect(*dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pgslot: connect: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := run(cmd, db, fs.Args(), *interval, *n); err != nil {
		fmt.Fprintf(os.Stderr, "pgslot: %v\n", err)
		os.Exit(1)
	}
}

func run(cmd string, db *sql.DB, args []string, interval time.Duration, n int) error {
	switch cmd {
	case "health":
		return cmdHealth(db, os.Stdout)
	case "slots":
		return cmdSlots(db, os.Stdout)
	case "watch":
		return cmdWatch(db, os.Stdout, interval)
	case "pipeline":
		return cmdPipeline(db, os.Stdout)
	case "pipeline-history":
		if len(args) < 1 {
			return fmt.Errorf("pipeline-history requires a slot name: pgslot pipeline-history <slot>")
		}
		return cmdPipelineHistory(db, os.Stdout, args[0], n)
	case "history":
		if len(args) < 1 {
			return fmt.Errorf("history requires a slot name: pgslot history <slot>")
		}
		return cmdHistory(db, os.Stdout, args[0], n)
	default:
		fmt.Fprint(os.Stderr, usage)
		os.Exit(1)
		return nil
	}
}
