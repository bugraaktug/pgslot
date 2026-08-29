# Walkrie

[Walkrie](https://github.com/bugraaktug/walkrie) is a native C++ daemon that
streams PostgreSQL logical replication changes into vector-embedding sinks
(pgvector, Qdrant, or JSON debug output) in real time. It's the reference
integration this repo's adapter contract (`adapters/README.md`) was
validated against -- everything below is real, run against a live cluster,
not illustrative.

## Config

Walkrie reports via its own `[observability]` config block (Walkrie-side
design doc: `PGSLOT_ADAPTER.md` in the Walkrie repo). One block covers every
`[[source]]` Walkrie is configured with -- each source/slot gets its own
`report_metric()` call per reporting tick:

```toml
[observability]
host               = "localhost"
port               = "5432"
dbname             = "qdb"
user               = "walkrie_adapter"
password           = "..."
adapter_name       = "walkrie"
report_interval_ms = 5000
```

A real multi-source Walkrie deployment on this host reports for three slots
this way: `cdc_slot` (publication `test_pub`, primary source), `pgcdc_slot`,
and `cdc_backfill_slot`.

## Role

```sql
CREATE ROLE walkrie_adapter LOGIN PASSWORD '...' IN ROLE pgslot_adapter;
```

`EXECUTE` on `report_metric()` only, per the generic contract -- Walkrie's
reporter never touches `adapter_metrics` directly and doesn't need to.

## What it looks like end-to-end

```
$ pgslot pipeline
SLOT          STATE     RETAINED  ADAPTER  PROCESSED LSN  EVENTS/SEC  QUEUE  SINK  LAST REPORT
cdc_slot      WARNING   27.8 MB   walkrie  0/40F01AC8     1           0      ok    2026-08-28 15:02:37
pgcdc_slot    WARNING   27.8 MB   walkrie  0/3F828410     0           0      ok    2026-08-28 14:37:21

$ pgslot pipeline-history cdc_slot
COLLECTED AT          ADAPTER  PROCESSED LSN  EVENTS/SEC  QUEUE  SINK
2026-08-28 14:37:21   walkrie  0/3C55D8A8     0           0      ok
2026-08-28 15:02:37   walkrie  0/40F01AC8     1           0      ok
```

## Troubleshooting

**`permission denied for table adapter_metrics`, or `relation
"adapter_metrics" does not exist`, when Walkrie calls `report_metric()`:**
this was a real bug in `report_metric()` itself (missing `SECURITY DEFINER`
+ `search_path` pin), not a Walkrie or role-setup problem -- fixed in
`sql/pgslot--0.3.sql`. If you're still seeing it, you're on an unfixed
install; upgrade (`ALTER EXTENSION pgslot UPDATE TO '0.3'` or later).

**`report_metric()` calls succeed but `pgslot pipeline`/`pipeline-history`
come back empty or grant errors:** check `pgslot_monitor`'s grants actually
include the views (`\dp pgslot.slot_pipeline` as a superuser) -- if pgslot's
extension schema was ever dropped and recreated on this cluster, view/
function ACLs reset to empty and need `scripts/roles.sql` re-run (safe to
re-run any time, it's idempotent).

**`events_per_sec` reads `0` for a `backfill = true` source even while
`queue_depth` is visibly draining:** not a bug. Walkrie's backfill drains
through a separate `walkrie_worker` process, not the main `EventDispatcher`
that `PgslotReporter`'s live event counter instruments, so that process's
throughput is architecturally invisible to the counter. `queue_depth` still
reads correctly during backfill (it's dispatcher-wide, not per-process) --
use it, not `events_per_sec`, to judge backfill progress. Confirmed against
`cdc_backfill_slot` in the 2026-08-28 live multi-source test.
