-- pgslot role setup.
--
-- Deliberately NOT shipped inside pgslot--0.1.sql: roles are cluster-wide
-- objects, not extension-owned ones. CREATE EXTENSION / DROP EXTENSION
-- should never create or delete a login-capable role as a side effect.
-- Run this once per cluster, after `CREATE EXTENSION pgslot;` -- and safe
-- to re-run any time after (e.g. after a DROP/CREATE EXTENSION cycle,
-- which recreates the views/functions with empty ACLs but leaves the
-- cluster-wide roles below untouched -- re-running just this file restores
-- the grants without erroring on the already-existing roles).
DO $$ BEGIN
    CREATE ROLE pgslot_monitor NOLOGIN;
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

GRANT USAGE ON SCHEMA pgslot TO pgslot_monitor;
GRANT SELECT ON
    pgslot.slot_current,
    pgslot.slot_rates,
    pgslot.slot_history_rates,
    pgslot.slot_health,
    pgslot.wal_summary,
    pgslot.slot_pipeline,
    pgslot.slot_pipeline_history,
    pgslot.available_publications
TO pgslot_monitor;

-- No SELECT on pgslot.slot_history or pgslot.adapter_metrics directly --
-- those are storage, not API. Views only.

-- ---------------------------------------------------------------------
-- Role for whatever calls pgslot.collect() / pgslot.prune() on a timer
-- (systemd service user, k8s CronJob service account, pgslot CLI, ...).
-- collect()/prune() are SECURITY DEFINER, so this role does NOT need
-- pg_monitor or superuser -- it only needs EXECUTE.
-- ---------------------------------------------------------------------
DO $$ BEGIN
    CREATE ROLE pgslot_collector NOLOGIN;
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

GRANT USAGE ON SCHEMA pgslot TO pgslot_collector;
GRANT EXECUTE ON FUNCTION pgslot.collect()          TO pgslot_collector;
GRANT EXECUTE ON FUNCTION pgslot.prune(integer)      TO pgslot_collector;

-- ---------------------------------------------------------------------
-- Base role for adapters (Walkrie, or anything else reporting pipeline
-- metrics -- e.g. a Debezium-based sink). report_metric() is generic across
-- adapter_name values, so every adapter shares this one NOLOGIN role;
-- create one LOGIN role per adapter *inheriting* it, e.g.:
--   CREATE ROLE walkrie_adapter LOGIN PASSWORD '...' IN ROLE pgslot_adapter;
-- Adapters get EXECUTE on report_metric() only -- no SELECT/INSERT on
-- adapter_metrics itself, so a compromised or buggy adapter can't read
-- other adapters' rows or touch anything else in the schema.
-- ---------------------------------------------------------------------
DO $$ BEGIN
    CREATE ROLE pgslot_adapter NOLOGIN;
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

GRANT USAGE ON SCHEMA pgslot TO pgslot_adapter;
GRANT EXECUTE ON FUNCTION pgslot.report_metric(text, name, jsonb) TO pgslot_adapter;

-- ---------------------------------------------------------------------
-- Example: wire an actual login role to each group role above.
-- ---------------------------------------------------------------------
-- CREATE ROLE grafana_reader   LOGIN PASSWORD '...' IN ROLE pgslot_monitor;
-- CREATE ROLE pgslot_cron      LOGIN PASSWORD '...' IN ROLE pgslot_collector;
--
-- -- Walkrie: reports for whichever slots its own [[source]] blocks cover
-- -- (e.g. cdc_slot, pgcdc_slot, cdc_backfill_slot) via its own
-- -- [observability] config block -- see adapters/README.md.
-- CREATE ROLE walkrie_adapter  LOGIN PASSWORD '...' IN ROLE pgslot_adapter;
--
-- -- Any other adapter (e.g. a Debezium-based sink reporting for its own
-- -- slot) just needs its own login role in the same group -- no pgslot-side
-- -- change required beyond this one line:
-- CREATE ROLE debezium_adapter LOGIN PASSWORD '...' IN ROLE pgslot_adapter;
