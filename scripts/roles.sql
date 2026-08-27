-- pgslot role setup.
--
-- Deliberately NOT shipped inside pgslot--0.1.sql: roles are cluster-wide
-- objects, not extension-owned ones. CREATE EXTENSION / DROP EXTENSION
-- should never create or delete a login-capable role as a side effect.
-- Run this once per cluster, after `CREATE EXTENSION pgslot;`.

-- ---------------------------------------------------------------------
-- Read-only role for dashboards / CLI / Grafana / pgAdmin, etc.
-- ---------------------------------------------------------------------
CREATE ROLE pgslot_monitor NOLOGIN;

GRANT USAGE ON SCHEMA pgslot TO pgslot_monitor;
GRANT SELECT ON
    pgslot.slot_current,
    pgslot.slot_rates,
    pgslot.slot_history_rates,
    pgslot.slot_health,
    pgslot.wal_summary,
    pgslot.slot_pipeline
TO pgslot_monitor;

-- No SELECT on pgslot.slot_history or pgslot.adapter_metrics directly --
-- those are storage, not API. Views only.

-- ---------------------------------------------------------------------
-- Role for whatever calls pgslot.collect() / pgslot.prune() on a timer
-- (systemd service user, k8s CronJob service account, pgslot CLI, ...).
-- collect()/prune() are SECURITY DEFINER, so this role does NOT need
-- pg_monitor or superuser -- it only needs EXECUTE.
-- ---------------------------------------------------------------------
CREATE ROLE pgslot_collector NOLOGIN;

GRANT USAGE ON SCHEMA pgslot TO pgslot_collector;
GRANT EXECUTE ON FUNCTION pgslot.collect()          TO pgslot_collector;
GRANT EXECUTE ON FUNCTION pgslot.prune(integer)      TO pgslot_collector;

-- ---------------------------------------------------------------------
-- Base role for adapters (Walkrie, or anything else reporting pipeline
-- metrics). Create one login role per adapter *inheriting* this, e.g.:
--   CREATE ROLE walkrie_adapter LOGIN PASSWORD '...' IN ROLE pgslot_adapter;
-- ---------------------------------------------------------------------
CREATE ROLE pgslot_adapter NOLOGIN;

GRANT USAGE ON SCHEMA pgslot TO pgslot_adapter;
GRANT EXECUTE ON FUNCTION pgslot.report_metric(text, name, jsonb) TO pgslot_adapter;

-- ---------------------------------------------------------------------
-- Example: wire an actual login role to each group role above.
-- ---------------------------------------------------------------------
-- CREATE ROLE grafana_reader   LOGIN PASSWORD '...' IN ROLE pgslot_monitor;
-- CREATE ROLE pgslot_cron      LOGIN PASSWORD '...' IN ROLE pgslot_collector;
-- CREATE ROLE walkrie_adapter  LOGIN PASSWORD '...' IN ROLE pgslot_adapter;
