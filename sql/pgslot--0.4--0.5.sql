-- pgslot--0.4--0.5.sql
-- Adds `active` to slot_pipeline. slot_health already carries active, but
-- slot_pipeline's SELECT list never re-exposed it, so a UI/CLI joining
-- pipeline data couldn't tell a CRITICAL-but-inactive slot (e.g. an
-- orphaned slot retaining WAL) from a CRITICAL-and-active one -- the two
-- facts are independent and both matter for triage. Appended as the last
-- column (Postgres requires CREATE OR REPLACE VIEW to preserve existing
-- column order; new columns can only be added at the end).
--
-- No table/column changes, no grants here -- roles are cluster-wide and
-- deliberately kept out of extension scripts (see scripts/roles.sql).
-- slot_pipeline is already granted to pgslot_monitor, so no re-grant is
-- needed for this upgrade.

\echo Use "ALTER EXTENSION pgslot UPDATE TO '0.5'" to apply this upgrade. \quit

CREATE OR REPLACE VIEW slot_pipeline AS
SELECT
    h.slot_name,
    h.status,
    h.reason,
    h.retained_bytes,
    a.adapter_name,
    a.metrics             AS adapter_metrics,
    a.collected_at         AS adapter_sample_at,
    h.active
FROM slot_health h
LEFT JOIN LATERAL (
    SELECT adapter_name, metrics, collected_at
    FROM adapter_metrics m
    WHERE m.slot_name = h.slot_name
    ORDER BY collected_at DESC
    LIMIT 1
) a ON true;

COMMENT ON VIEW slot_pipeline IS
    'slot_health joined against the latest adapter-reported metrics, e.g. '
    'Walkrie''s processed_lsn/events_per_sec alongside Postgres-side slot lag. '
    'Includes active separately from status: a slot can be critical while '
    'still active, or critical because it is inactive -- the two are not '
    'the same fact.';
