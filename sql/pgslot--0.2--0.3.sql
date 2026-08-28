-- pgslot--0.2--0.3.sql
-- Adds slot_pipeline_history: the full per-report time series behind
-- slot_pipeline's latest-only adapter_metrics join -- needed so a UI/CLI
-- can chart adapter-reported metrics (events_per_sec, queue_depth, ...)
-- over time instead of only ever seeing the latest sample. Same
-- relationship slot_history_rates already has to slot_rates.
--
-- No table/column changes, no grants here -- roles are cluster-wide and
-- deliberately kept out of extension scripts (see scripts/roles.sql); after
-- this upgrade, re-run scripts/roles.sql to add SELECT on the new view to
-- pgslot_monitor (it's idempotent, safe to re-run any time).

\echo Use "ALTER EXTENSION pgslot UPDATE TO '0.3'" to apply this upgrade. \quit

CREATE VIEW slot_pipeline_history AS
SELECT
    m.slot_name,
    m.adapter_name,
    m.metrics,
    m.collected_at
FROM adapter_metrics m
ORDER BY slot_name, collected_at DESC;

COMMENT ON VIEW slot_pipeline_history IS
    'Full per-report time series of adapter-reported metrics (all adapters, '
    'all slots) -- the history behind slot_pipeline''s latest-only join, same '
    'relationship slot_history_rates has to slot_rates. Feeds '
    '`pgslot pipeline-history <slot>`. Runs on its own time axis -- adapter '
    'report ticks and pgslot collect() ticks are on independent schedules, '
    'so don''t assume timestamps here line up with slot_history_rates rows.';
