-- Smoke test: install, collect, check views resolve, uninstall cleanly.
-- Run against a scratch database: psql -d scratch -f test/sql/basic.sql

CREATE EXTENSION pgslot;

-- collect() should succeed even with zero replication slots present
SELECT pgslot.collect() AS rows_collected;

-- views should all resolve without error regardless of row count
SELECT count(*) FROM pgslot.slot_current;
SELECT count(*) FROM pgslot.slot_rates;
SELECT count(*) FROM pgslot.slot_health;
SELECT count(*) FROM pgslot.wal_summary;
SELECT count(*) FROM pgslot.slot_pipeline;

-- adapter reporting path
SELECT pgslot.report_metric('test_adapter', 'nonexistent_slot',
                             jsonb_build_object('events_per_sec', 0));
SELECT count(*) FROM pgslot.adapter_metrics WHERE adapter_name = 'test_adapter';

-- prune should run without error on an empty/near-empty table
SELECT pgslot.prune(0) AS rows_pruned;

DROP EXTENSION pgslot;
