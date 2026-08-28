# pgslot CLI

A Go binary over pgslot's read-only views -- never touches the raw tables,
never mutates a slot. Same read-only contract as the SQL extension itself.

## Build

```bash
cd cli && make build      # ./pgslot
make install               # installs to $PREFIX/bin (default /usr/local/bin)
```

Ships as a single static binary -- no runtime dependency for whoever installs
it. Kept out of the top-level `Makefile`, same reasoning as `src/Makefile`:
the extension's own `make install` stays pure SQL, no Go toolchain needed
just to install pgslot itself.

## Connect

Uses the standard `PG*` environment variables (`PGHOST`, `PGUSER`,
`PGDATABASE`, ...), same as `psql`, or a full connection string via
`-dsn` / `PGSLOT_DSN`. Connect as a role granted `pgslot_monitor`
(see `../scripts/roles.sql`) -- the CLI only ever needs `SELECT` on the views.

## Commands

```bash
pgslot health              # cluster-wide summary
pgslot slots                # per-slot status table
pgslot watch [-interval 2s] # auto-refreshing status table
pgslot history <slot> [-n 20]  # per-slot snapshot history
pgslot pipeline              # per-slot status joined with adapter-reported metrics
pgslot pipeline-history <slot> [-n 20]  # per-slot adapter-reported metrics history
```

```
SLOT            STATE      WAL DISTANCE    RATE
walkrie         HEALTHY    1.2 GB          -12 MB/s
debezium        WARNING    82 GB           +43 MB/s
old_slot        CRITICAL   412 GB          +0 MB/s
```

RATE is net WAL retention change (`wal_growth_bytes_per_sec -
consumer_bytes_per_sec`): negative means the consumer is catching up,
positive means retained WAL is growing.

`pgslot history` reads `pgslot.slot_history_rates` -- the full per-snapshot
time series behind `slot_rates`, which only exposes each slot's latest rate.

`pgslot pipeline` reads `pgslot.slot_pipeline` -- Postgres-side slot health
next to whatever the slot's adapter (Walkrie, or any other `pgslot_adapter`
member) last reported via `report_metric()`:

```
SLOT       STATE      RETAINED  ADAPTER  PROCESSED LSN  EVENTS/SEC  QUEUE  SINK  LAST REPORT
cdc_slot   WARNING    23.4 MB   walkrie  0/40F01AC8     1           0      ok    2026-08-28 15:02:37
old_slot   CRITICAL   412 GB    n/a      n/a            n/a         n/a    n/a   n/a
```

`ADAPTER`/`PROCESSED LSN`/`EVENTS/SEC`/`QUEUE`/`SINK` all read "n/a" when no
adapter has reported for a slot yet -- `adapter_metrics` is schema-free JSON
by design (see `../adapters/README.md`), so any of these can legitimately be
absent even once an adapter is reporting; the CLI only ever renders the four
keys Walkrie's `PgslotReporter` happens to send today, falling back to "n/a"
for anything else.

`pgslot pipeline-history <slot>` reads `pgslot.slot_pipeline_history` -- the
full per-report time series behind `slot_pipeline`'s latest-only join, same
relationship `pgslot history` has to `slot_current`. Useful for actually
seeing an adapter's `events_per_sec`/`queue_depth` trend over time instead of
only the latest sample. Runs on its own time axis: adapter report ticks and
pgslot's own `collect()` ticks are independently scheduled, so don't expect
its timestamps to line up row-for-row with `pgslot history`'s output for the
same slot.
