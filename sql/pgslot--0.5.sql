-- pgslot--0.5.sql
-- Replication slot WAL health, history, and diagnostics.
--
-- Design notes:
--   * No slot-mutating actions (no drop/advance/create). Read + diagnose only.
--   * Collection is pgslot.collect(), a plain SQL/PLpgSQL function -- this
--     file alone needs no C compile step and no restart. The recommended
--     way to actually run it is src/'s background worker (shared_preload_
--     libraries, restart required); calling it yourself on an external
--     schedule (cron/systemd/k8s CronJob) remains a supported fallback for
--     anyone who doesn't want to restart Postgres. See src/README.md.
--   * Raw tables are not exposed directly; all read access goes through
--     views, so the storage layout can change across versions without
--     breaking consumers.
--   * SECURITY DEFINER functions pin search_path explicitly to avoid
--     search_path hijacking via a malicious schema earlier in the caller's path.

\echo Use "CREATE EXTENSION pgslot" to load this file. \quit

-- =========================================================================
-- 1. Storage
-- =========================================================================

CREATE TABLE slot_history (
    collected_at         timestamptz NOT NULL DEFAULT clock_timestamp(),
    slot_name             name        NOT NULL,
    slot_type             text        NOT NULL,
    plugin                 text,
    database                name,
    active                  boolean     NOT NULL,
    active_pid              integer,
    restart_lsn             pg_lsn,
    confirmed_flush_lsn     pg_lsn,
    current_wal_lsn         pg_lsn      NOT NULL,
    wal_status               text,
    safe_wal_size            bigint,
    retained_bytes           bigint,
    unflushed_bytes          bigint,
    PRIMARY KEY (slot_name, collected_at)
);

COMMENT ON TABLE slot_history IS
    'Raw periodic snapshots of pg_replication_slots. Not exposed directly -- '
    'read through pgslot.slot_current / slot_rates / slot_health instead.';

-- Supports "all slots over a time range" queries (wal_summary trends, pruning).
CREATE INDEX slot_history_time_idx ON slot_history (collected_at);

REVOKE ALL ON slot_history FROM PUBLIC;

-- Free-form metrics reported by external adapters (e.g. Walkrie) so pipeline
-- state can be joined against slot state without pgslot knowing anything
-- about the adapter's internals.
CREATE TABLE adapter_metrics (
    collected_at    timestamptz NOT NULL DEFAULT clock_timestamp(),
    adapter_name     text        NOT NULL,
    slot_name         name        NOT NULL,
    metrics            jsonb       NOT NULL,
    PRIMARY KEY (adapter_name, slot_name, collected_at)
);

COMMENT ON TABLE adapter_metrics IS
    'Adapter-reported metrics (e.g. processed_lsn, events_sec, queue_depth) '
    'keyed by slot_name so they can be joined against slot_health. '
    'See adapters/ in the source tree for the reporting contract.';

CREATE INDEX adapter_metrics_slot_idx ON adapter_metrics (slot_name, collected_at);

REVOKE ALL ON adapter_metrics FROM PUBLIC;

-- =========================================================================
-- 2. Collection
-- =========================================================================

CREATE FUNCTION collect()
RETURNS integer
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pgslot, pg_catalog
AS $$
DECLARE
    v_wal_lsn pg_lsn;
    v_count   integer;
BEGIN
    v_wal_lsn := pg_current_wal_lsn();

    INSERT INTO slot_history (
        slot_name, slot_type, plugin, database, active, active_pid,
        restart_lsn, confirmed_flush_lsn, current_wal_lsn,
        wal_status, safe_wal_size, retained_bytes, unflushed_bytes
    )
    SELECT
        s.slot_name, s.slot_type, s.plugin, s.database, s.active, s.active_pid,
        s.restart_lsn, s.confirmed_flush_lsn, v_wal_lsn,
        s.wal_status, s.safe_wal_size,
        pg_wal_lsn_diff(v_wal_lsn, s.restart_lsn)::bigint,
        pg_wal_lsn_diff(v_wal_lsn, s.confirmed_flush_lsn)::bigint
    FROM pg_replication_slots s;

    GET DIAGNOSTICS v_count = ROW_COUNT;
    RETURN v_count;
END;
$$;

COMMENT ON FUNCTION collect() IS
    'Snapshot pg_replication_slots into slot_history. Call on an interval '
    'from systemd/cron/k8s CronJob/pgslot CLI. Idempotent per call, no locking '
    'beyond a normal INSERT.';

REVOKE ALL ON FUNCTION collect() FROM PUBLIC;

CREATE FUNCTION prune(keep_hours integer DEFAULT 168)
RETURNS bigint
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pgslot, pg_catalog
AS $$
DECLARE
    v_deleted bigint;
BEGIN
    DELETE FROM slot_history
    WHERE collected_at < clock_timestamp() - make_interval(hours => keep_hours);
    GET DIAGNOSTICS v_deleted = ROW_COUNT;
    RETURN v_deleted;
END;
$$;

COMMENT ON FUNCTION prune(integer) IS
    'Delete slot_history rows older than keep_hours (default 7 days). '
    'Run on the same schedule as collect(); pgslot does not do this for you.';

REVOKE ALL ON FUNCTION prune(integer) FROM PUBLIC;

-- =========================================================================
-- 3. Adapter reporting contract
-- =========================================================================

CREATE FUNCTION report_metric(p_adapter_name text, p_slot_name name, p_metrics jsonb)
RETURNS void
LANGUAGE sql
SECURITY DEFINER
SET search_path = pgslot, pg_catalog
AS $$
    INSERT INTO adapter_metrics (adapter_name, slot_name, metrics)
    VALUES (p_adapter_name, p_slot_name, p_metrics);
$$;

REVOKE ALL ON FUNCTION report_metric(text, name, jsonb) FROM PUBLIC;

COMMENT ON FUNCTION report_metric(text, name, jsonb) IS
    'Adapter entry point. e.g.: SELECT pgslot.report_metric(''walkrie'', '
    '''walkrie_cdc'', jsonb_build_object(''processed_lsn'', pg_current_wal_lsn()::text, '
    '''events_per_sec'', 1420, ''queue_depth'', 12)). SECURITY DEFINER (like '
    'collect()/prune()) with search_path pinned to pgslot, pg_catalog -- so '
    'EXECUTE alone is sufficient for an adapter role and adapter_metrics stays '
    'unreachable directly. Grant EXECUTE to the adapter''s own role, not to '
    'pgslot_monitor.';

-- =========================================================================
-- 4. Diagnostic views (read surface)
-- =========================================================================

CREATE VIEW slot_current AS
SELECT DISTINCT ON (slot_name) *
FROM slot_history
ORDER BY slot_name, collected_at DESC;

COMMENT ON VIEW slot_current IS 'Most recent snapshot per slot.';

CREATE VIEW slot_history_rates AS
WITH ranked AS (
    SELECT
        *,
        lag(collected_at)         OVER w AS prev_collected_at,
        lag(current_wal_lsn)      OVER w AS prev_wal_lsn,
        lag(confirmed_flush_lsn)  OVER w AS prev_confirmed_flush_lsn
    FROM slot_history
    WINDOW w AS (PARTITION BY slot_name ORDER BY collected_at)
)
SELECT
    slot_name,
    collected_at,
    active,
    wal_status,
    retained_bytes,
    prev_collected_at,
    EXTRACT(EPOCH FROM (collected_at - prev_collected_at)) AS interval_secs,
    CASE WHEN prev_collected_at IS NOT NULL AND collected_at > prev_collected_at THEN
        round(
            pg_wal_lsn_diff(current_wal_lsn, prev_wal_lsn)::numeric
            / EXTRACT(EPOCH FROM (collected_at - prev_collected_at)), 2)
    END AS wal_growth_bytes_per_sec,
    CASE WHEN prev_collected_at IS NOT NULL AND collected_at > prev_collected_at THEN
        round(
            pg_wal_lsn_diff(confirmed_flush_lsn, prev_confirmed_flush_lsn)::numeric
            / EXTRACT(EPOCH FROM (collected_at - prev_collected_at)), 2)
    END AS consumer_bytes_per_sec
FROM ranked
ORDER BY slot_name, collected_at DESC;

COMMENT ON VIEW slot_history_rates IS
    'Per-snapshot WAL growth vs. consumer progress rate for every collect() '
    'tick, not just the latest -- the time series behind slot_rates. Feeds '
    '`pgslot history <slot>`.';

CREATE VIEW slot_rates AS
SELECT DISTINCT ON (slot_name) *
FROM slot_history_rates
ORDER BY slot_name, collected_at DESC;

COMMENT ON VIEW slot_rates IS
    'WAL growth vs. consumer progress rate, computed from actual elapsed '
    'time between the two most recent snapshots (not an assumed interval).';

CREATE VIEW slot_health AS
SELECT
    c.slot_name,
    c.slot_type,
    c.plugin,
    c.database,
    c.active,
    c.wal_status,
    c.retained_bytes,
    c.safe_wal_size,
    r.wal_growth_bytes_per_sec,
    r.consumer_bytes_per_sec,
    CASE
        WHEN NOT c.active AND c.retained_bytes > 100 * 1024 * 1024
            THEN 'critical'
        WHEN c.wal_status IN ('extended', 'unreserved', 'lost')
            THEN 'critical'
        WHEN r.wal_growth_bytes_per_sec IS NOT NULL
             AND r.consumer_bytes_per_sec IS NOT NULL
             AND r.consumer_bytes_per_sec < r.wal_growth_bytes_per_sec * 0.5
            THEN 'warning'
        ELSE 'ok'
    END AS status,
    CASE
        WHEN NOT c.active AND c.retained_bytes > 100 * 1024 * 1024 THEN
            format('inactive slot retaining %s MB of WAL',
                   round(c.retained_bytes / 1048576.0, 1))
        WHEN c.wal_status IN ('extended', 'unreserved', 'lost') THEN
            format('wal_status is %s', c.wal_status)
        WHEN r.wal_growth_bytes_per_sec IS NOT NULL
             AND r.consumer_bytes_per_sec IS NOT NULL
             AND r.consumer_bytes_per_sec < r.wal_growth_bytes_per_sec * 0.5 THEN
            format('consumer processing %s B/s against %s B/s WAL growth',
                   r.consumer_bytes_per_sec, r.wal_growth_bytes_per_sec)
        ELSE 'nominal'
    END AS reason,
    c.collected_at AS last_sample_at
FROM slot_current c
LEFT JOIN slot_rates r USING (slot_name);

COMMENT ON VIEW slot_health IS
    'Status + human-readable reason per slot. This is the primary view a '
    'dashboard or CLI should query. Read-only -- no remediation actions.';

CREATE VIEW wal_summary AS
SELECT
    count(*)                                   AS slot_count,
    count(*) FILTER (WHERE active)              AS active_count,
    sum(retained_bytes)                          AS total_retained_bytes,
    max(retained_bytes)                          AS max_retained_bytes,
    (SELECT slot_name FROM slot_current
       ORDER BY retained_bytes DESC NULLS LAST LIMIT 1) AS top_consumer_slot
FROM slot_current;

COMMENT ON VIEW wal_summary IS 'Cluster-wide slot totals, for a single dashboard header row.';

CREATE VIEW slot_pipeline AS
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

CREATE VIEW available_publications AS
SELECT
    p.pubname,
    p.pubowner::regrole::text AS owner,
    p.puballtables,
    p.pubinsert,
    p.pubupdate,
    p.pubdelete,
    p.pubtruncate,
    p.pubviaroot
FROM pg_publication p
ORDER BY p.pubname;

COMMENT ON VIEW available_publications IS
    'Read-only wrapper over pg_publication -- informational only, cannot '
    'be joined to slot_health since pg_replication_slots does not record '
    'which publication a slot consumes (that mapping lives only in '
    'subscriber/adapter config).';
