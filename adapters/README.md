# Adapter contract

An adapter is anything downstream of a replication slot that wants its own
pipeline state (not just Postgres's view of the slot) joined against
`pgslot.slot_health`. pgslot knows nothing about adapter internals -- it just
stores whatever JSON the adapter reports, keyed by `slot_name`.

## Reporting

```sql
SELECT pgslot.report_metric(
    'walkrie',                 -- adapter_name: your own free-form identifier
    'walkrie_cdc',              -- slot_name: must match the pg_replication_slots row
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
`slot_pipeline` consumers can rely on them.

## Reading

```sql
SELECT slot_name, status, reason, retained_bytes, adapter_metrics
FROM pgslot.slot_pipeline
WHERE slot_name = 'walkrie_cdc';
```

`slot_pipeline` left-joins the latest `adapter_metrics` row onto
`slot_health`, so you get Postgres-side lag (`retained_bytes`, `status`) next
to adapter-side progress (`adapter_metrics->>'processed_lsn'`,
`adapter_metrics->>'events_per_sec'`) in one row -- enough to tell whether a
pipeline is behind because Postgres is generating WAL fast, or because the
consumer itself has stalled.

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
