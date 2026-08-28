# Adapter contract

An adapter is anything downstream of a replication slot that wants its own
pipeline state (not just Postgres's view of the slot) joined against
`pgslot.slot_health`. pgslot knows nothing about adapter internals -- it just
stores whatever JSON the adapter reports, keyed by `slot_name`.

This page is the generic contract only. For a concrete, real worked example
against an actual running adapter, see:

- [`walkrie.md`](walkrie.md) -- [Walkrie](https://github.com/bugraaktug/walkrie)
  (a Postgres WAL-to-vector-embedding sync engine), the reference
  integration this contract was validated against.

## Reporting

```sql
SELECT pgslot.report_metric(
    'myadapter',                -- adapter_name: your own free-form identifier
    'my_slot',                   -- slot_name: must match the pg_replication_slots row
    jsonb_build_object(
        'processed_lsn',   pg_current_wal_lsn()::text,
        'events_per_sec',  1420,
        'queue_depth',     12,
        'sink_status',     'ok'
    )
);
```

Call this on whatever cadence makes sense for the adapter -- it does not need
to match pgslot's own `collect()` interval. There's no schema enforced on the
`jsonb` payload beyond "valid JSON"; keep keys stable across calls so
`slot_pipeline`/`slot_pipeline_history` consumers can rely on them.
`report_metric()` is a plain `INSERT` wrapper -- adapters have no
`SELECT`/`INSERT` grant on `adapter_metrics` itself (`REVOKE ALL ... FROM
PUBLIC`), so this function is the only way in. An adapter with multiple
sources/slots (e.g. Walkrie running several `[[source]]` blocks) just means
multiple calls, one per slot, each tick.

## Reading

```sql
-- latest sample per slot
SELECT slot_name, status, reason, retained_bytes, adapter_metrics
FROM pgslot.slot_pipeline
WHERE slot_name = 'my_slot';

-- full history for one slot, e.g. to chart events_per_sec/queue_depth over time
SELECT adapter_name, metrics, collected_at
FROM pgslot.slot_pipeline_history
WHERE slot_name = 'my_slot'
ORDER BY collected_at DESC;
```

`slot_pipeline` left-joins the latest `adapter_metrics` row onto
`slot_health`, so you get Postgres-side lag (`retained_bytes`, `status`) next
to adapter-side progress (`adapter_metrics->>'processed_lsn'`,
`adapter_metrics->>'events_per_sec'`) in one row -- enough to tell whether a
pipeline is behind because Postgres is generating WAL fast, or because the
consumer itself has stalled. The join picks the **latest row by `slot_name`
alone** -- `adapter_name` is not part of it. If two different adapters (say,
Walkrie and a Debezium-based one) both report against the same `slot_name`,
`slot_pipeline` only ever shows whichever reported most recently; full
history for both is still kept in `adapter_metrics`, just not surfaced by
that view. Not a problem when each slot has one adapter, which is the
expected case -- don't build a consumer that assumes this view merges
multiple adapters' data for one slot.

`slot_pipeline_history` is that same data as a full time series instead of
latest-only -- same relationship `slot_history_rates` has to `slot_rates` --
because `slot_pipeline` alone isn't enough to chart a trend. It runs on its
own time axis: adapter report ticks and pgslot's own `collect()` ticks are
independently scheduled, so don't expect its timestamps to line up with
`slot_history_rates`' for the same slot.

## Stability contract

Two independent one-way promises meet at `report_metric()`, so pgslot's and
an adapter's release cycles never need to block each other:

- **pgslot's promise to adapters:** `report_metric()`'s signature
  (`text, name, jsonb`) and the `pgslot_adapter` role's `EXECUTE` grant on it
  won't change in a breaking way. Everything else on this side --
  `adapter_metrics`' columns/indexes, `slot_pipeline`'s join logic,
  `slot_health`'s thresholds, new views entirely -- is free to change at any
  time, because adapters never read any of it back; this is a
  write-and-forget call. If the signature itself ever must change, expect a
  new overload (Postgres allows this), not an in-place edit.
- **An adapter's promise to pgslot's consumers:** the JSON key names it
  writes (e.g. Walkrie's `processed_lsn`/`events_per_sec`/`queue_depth`/
  `sink_status`) are that adapter's own contract with whoever reads
  `slot_pipeline` -- pgslot itself enforces nothing beyond "valid JSON".
  Adding new keys later is free; renaming or repurposing an existing one
  is a breaking change for that adapter's own dashboards, not pgslot's
  problem to solve.

## Grants

Adapters need their own login role in `pgslot_adapter` (see
`scripts/roles.sql`):

```sql
CREATE ROLE walkrie_adapter LOGIN PASSWORD '...' IN ROLE pgslot_adapter;
```

`pgslot_adapter` only grants `EXECUTE` on `report_metric()` -- adapters
cannot read `slot_history`, other adapters' rows, or anything else in the
`pgslot` schema. If an adapter also needs to read slot health (e.g. to decide
whether to back off), grant it `pgslot_monitor` too.
